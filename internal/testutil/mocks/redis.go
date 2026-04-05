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
func NewMockRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()

	// miniredis.RunT automatically handles cleanup via t.Cleanup
	mr := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return mr, client
}
