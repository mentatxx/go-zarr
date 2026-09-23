package store

import (
	"context"
	"fmt"
	"iter"
	"strings"
	"sync"
)

// Memory is an in-memory listable store.
type Memory struct {
	mu sync.RWMutex
	m  map[string][]byte
}

// NewMemory creates an empty memory store.
func NewMemory() *Memory {
	return &Memory{m: make(map[string][]byte)}
}

func memKey(keys []string) string {
	return strings.Join(JoinKeys(keys), "\x00")
}

func (s *Memory) Exists(ctx context.Context, keys []string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.m[memKey(keys)]
	return ok, nil
}

func (s *Memory) Get(ctx context.Context, keys []string, start, end int64) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.m[memKey(keys)]
	if !ok {
		return nil, nil
	}
	if end < 0 {
		end = int64(len(b))
	}
	if start < 0 {
		start = int64(len(b)) + start
	}
	if start < 0 {
		start = 0
	}
	if end > int64(len(b)) {
		end = int64(len(b))
	}
	if start > end {
		return []byte{}, nil
	}
	out := make([]byte, end-start)
	copy(out, b[start:end])
	return out, nil
}

func (s *Memory) Set(ctx context.Context, keys []string, data []byte) error {
	cp := make([]byte, len(data))
	copy(cp, data)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[memKey(keys)] = cp
	return nil
}

func (s *Memory) Delete(ctx context.Context, keys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, memKey(keys))
	return nil
}

func (s *Memory) Size(ctx context.Context, keys []string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.m[memKey(keys)]
	if !ok {
		return -1, nil
	}
	return int64(len(b)), nil
}

func (s *Memory) List(ctx context.Context, prefix []string) iter.Seq2[[]string, error] {
	pref := JoinKeys(prefix)
	return func(yield func([]string, error) bool) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		for k := range s.m {
			parts := strings.Split(k, "\x00")
			if !hasPrefix(parts, pref) {
				continue
			}
			rel := parts[len(pref):]
			if !yield(rel, nil) {
				return
			}
		}
	}
}

func (s *Memory) ListChildren(ctx context.Context, prefix []string) iter.Seq2[[]string, error] {
	pref := JoinKeys(prefix)
	return func(yield func([]string, error) bool) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		seen := map[string]struct{}{}
		for k := range s.m {
			parts := strings.Split(k, "\x00")
			if len(parts) <= len(pref) || !hasPrefix(parts, pref) {
				continue
			}
			child := parts[len(pref)]
			if _, ok := seen[child]; ok {
				continue
			}
			seen[child] = struct{}{}
			if !yield([]string{child}, nil) {
				return
			}
		}
	}
}

func hasPrefix(parts, pref []string) bool {
	if len(parts) < len(pref) {
		return false
	}
	for i := range pref {
		if parts[i] != pref[i] {
			return false
		}
	}
	return true
}

func (s *Memory) String() string {
	return fmt.Sprintf("<MemoryStore {%p}>", s)
}

// Resolve returns a handle at keys.
func (s *Memory) Resolve(keys ...string) Handle {
	return NewHandle(s, keys...)
}
