package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/mentatxx/go-zarr/store"
	"github.com/stretchr/testify/require"
)

func TestS3Store(t *testing.T) {
	if os.Getenv("RUN_S3_TESTS") != "1" {
		t.Skip("set RUN_S3_TESTS=1 and start s3mock on :9090")
	}
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:9090"
	}
	client := s3.New(s3.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(endpoint),
		Credentials:  aws.AnonymousCredentials{},
		UsePathStyle: true,
	})
	s := store.NewS3(client, "zarr-test-bucket", "go-zarr")
	testStoreBasics(t, s, true)
	ctx := context.Background()
	require.NoError(t, s.Set(ctx, []string{"a", "b"}, []byte("x")))
	ok, err := s.Exists(ctx, []string{"a", "b"})
	require.NoError(t, err)
	require.True(t, ok)
}
