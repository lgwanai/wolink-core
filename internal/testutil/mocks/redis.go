// Package mocks provides reusable mock implementations for testing.
package mocks

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

// NewMockRedis creates an in-memory Redis server and client for testing.
// The server automatically cleans up when the test completes.
//
// Usage:
//
//	func TestSomething(t *testing.T) {
//	    mini, client := mocks.NewMockRedis(t)
//	    // Use client as redis.Client
//	    // mini provides direct access to in-memory data
//	}
//
// Can also be used with *testing.B for benchmarks:
//
//	func BenchmarkSomething(b *testing.B) {
//	    mini, client := mocks.NewMockRedis(b)
//	}
func NewMockRedis(t testing.TB) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()

	// miniredis.RunT automatically handles cleanup via t.Cleanup
	// RunT accepts miniredis.Tester interface which both *testing.T and *testing.B implement
	mr := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return mr, client
}
