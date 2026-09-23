package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3 is an S3-backed listable store.
type S3 struct {
	Client *s3.Client
	Bucket string
	Prefix string
}

// NewS3 creates an S3 store.
func NewS3(client *s3.Client, bucket, prefix string) *S3 {
	return &S3{Client: client, Bucket: bucket, Prefix: strings.Trim(prefix, "/")}
}

func (s *S3) key(keys []string) string {
	p := strings.Join(JoinKeys(keys), "/")
	if s.Prefix == "" {
		return p
	}
	if p == "" {
		return s.Prefix
	}
	return s.Prefix + "/" + p
}

func (s *S3) Exists(ctx context.Context, keys []string) (bool, error) {
	_, err := s.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.key(keys)),
	})
	if err != nil {
		var nf *types.NotFound
		var nsk *types.NoSuchKey
		if errors.As(err, &nf) || errors.As(err, &nsk) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *S3) Get(ctx context.Context, keys []string, start, end int64) ([]byte, error) {
	in := &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.key(keys)),
	}
	if start != 0 || end >= 0 {
		if end < 0 {
			if start < 0 {
				in.Range = aws.String(fmt.Sprintf("bytes=%d", start))
			} else {
				in.Range = aws.String(fmt.Sprintf("bytes=%d-", start))
			}
		} else {
			in.Range = aws.String(fmt.Sprintf("bytes=%d-%d", start, end-1))
		}
	}
	out, err := s.Client.GetObject(ctx, in)
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, nil
		}
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func (s *S3) Set(ctx context.Context, keys []string, data []byte) error {
	_, err := s.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.key(keys)),
		Body:   bytes.NewReader(data),
	})
	return err
}

func (s *S3) Delete(ctx context.Context, keys []string) error {
	_, err := s.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.key(keys)),
	})
	return err
}

func (s *S3) Size(ctx context.Context, keys []string) (int64, error) {
	out, err := s.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.key(keys)),
	})
	if err != nil {
		var nf *types.NotFound
		var nsk *types.NoSuchKey
		if errors.As(err, &nf) || errors.As(err, &nsk) {
			return -1, nil
		}
		return 0, err
	}
	if out.ContentLength != nil {
		return *out.ContentLength, nil
	}
	return 0, nil
}

func (s *S3) List(ctx context.Context, prefix []string) iter.Seq2[[]string, error] {
	return func(yield func([]string, error) bool) {
		p := s.key(prefix)
		if p != "" && !strings.HasSuffix(p, "/") {
			p += "/"
		}
		var token *string
		for {
			out, err := s.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
				Bucket:            aws.String(s.Bucket),
				Prefix:            aws.String(p),
				ContinuationToken: token,
			})
			if err != nil {
				yield(nil, err)
				return
			}
			for _, obj := range out.Contents {
				if obj.Key == nil {
					continue
				}
				rel := strings.TrimPrefix(*obj.Key, p)
				if rel == "" {
					continue
				}
				if !yield(strings.Split(rel, "/"), nil) {
					return
				}
			}
			if out.IsTruncated == nil || !*out.IsTruncated {
				return
			}
			token = out.NextContinuationToken
		}
	}
}

func (s *S3) ListChildren(ctx context.Context, prefix []string) iter.Seq2[[]string, error] {
	return func(yield func([]string, error) bool) {
		p := s.key(prefix)
		if p != "" && !strings.HasSuffix(p, "/") {
			p += "/"
		}
		var token *string
		for {
			out, err := s.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
				Bucket:            aws.String(s.Bucket),
				Prefix:            aws.String(p),
				Delimiter:         aws.String("/"),
				ContinuationToken: token,
			})
			if err != nil {
				yield(nil, err)
				return
			}
			for _, cp := range out.CommonPrefixes {
				if cp.Prefix == nil {
					continue
				}
				name := strings.TrimSuffix(strings.TrimPrefix(*cp.Prefix, p), "/")
				if name == "" {
					continue
				}
				if !yield([]string{name}, nil) {
					return
				}
			}
			for _, obj := range out.Contents {
				if obj.Key == nil {
					continue
				}
				rel := strings.TrimPrefix(*obj.Key, p)
				if rel == "" || strings.Contains(rel, "/") {
					continue
				}
				if !yield([]string{rel}, nil) {
					return
				}
			}
			if out.IsTruncated == nil || !*out.IsTruncated {
				return
			}
			token = out.NextContinuationToken
		}
	}
}

func (s *S3) String() string {
	return fmt.Sprintf("s3://%s/%s", s.Bucket, s.Prefix)
}

func (s *S3) Resolve(keys ...string) Handle {
	return NewHandle(s, keys...)
}
