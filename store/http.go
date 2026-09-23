package store

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTP is a read-only HTTP store.
type HTTP struct {
	Base   string
	Client *http.Client
}

// NewHTTP creates an HTTP store. timeoutSeconds of 0 uses 60s.
func NewHTTP(base string) *HTTP {
	return &HTTP{
		Base: strings.TrimRight(base, "/"),
		Client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (s *HTTP) url(keys []string) (string, error) {
	u, err := url.Parse(s.Base)
	if err != nil {
		return "", err
	}
	for _, k := range JoinKeys(keys) {
		u = u.JoinPath(k)
	}
	return u.String(), nil
}

func (s *HTTP) do(ctx context.Context, method, rawURL, rangeHdr string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if rangeHdr != "" {
		req.Header.Set("Range", rangeHdr)
	}
	return s.Client.Do(req)
}

func (s *HTTP) Exists(ctx context.Context, keys []string) (bool, error) {
	raw, err := s.url(keys)
	if err != nil {
		return false, err
	}
	resp, err := s.do(ctx, http.MethodHead, raw, "")
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300, nil
}

func (s *HTTP) Get(ctx context.Context, keys []string, start, end int64) ([]byte, error) {
	raw, err := s.url(keys)
	if err != nil {
		return nil, err
	}
	rangeHdr := ""
	if start != 0 || end >= 0 {
		if end < 0 {
			if start < 0 {
				rangeHdr = fmt.Sprintf("bytes=%d", start)
			} else {
				rangeHdr = fmt.Sprintf("bytes=%d-", start)
			}
		} else {
			if start < 0 {
				return nil, fmt.Errorf("store: start must be non-negative when end is set")
			}
			rangeHdr = fmt.Sprintf("bytes=%d-%d", start, end-1)
		}
	}
	resp, err := s.do(ctx, http.MethodGet, raw, rangeHdr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("store: HTTP GET %s: status %d", raw, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (s *HTTP) Set(ctx context.Context, keys []string, data []byte) error {
	return fmt.Errorf("store: HTTP store is read-only")
}

func (s *HTTP) Delete(ctx context.Context, keys []string) error {
	return fmt.Errorf("store: HTTP store is read-only")
}

func (s *HTTP) Size(ctx context.Context, keys []string) (int64, error) {
	raw, err := s.url(keys)
	if err != nil {
		return 0, err
	}
	resp, err := s.do(ctx, http.MethodHead, raw, "")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return -1, nil
	}
	if resp.ContentLength >= 0 {
		return resp.ContentLength, nil
	}
	data, err := s.Get(ctx, keys, 0, -1)
	if err != nil {
		return 0, err
	}
	if data == nil {
		return -1, nil
	}
	return int64(len(data)), nil
}

func (s *HTTP) String() string { return s.Base }

// Resolve returns a handle at keys.
func (s *HTTP) Resolve(keys ...string) Handle {
	return NewHandle(s, keys...)
}
