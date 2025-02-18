package middleware

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	cache := NewCache()

	// Test setting and getting a value
	cache.Set("key1", "value1", 1*time.Minute)
	value, found := cache.Get("key1")
	if !found {
		t.Error("Expected to find key1")
	}
	if value != "value1" {
		t.Errorf("Expected value1, got %v", value)
	}

	// Test getting a non-existent key
	_, found = cache.Get("nonexistent")
	if found {
		t.Error("Expected not to find nonexistent key")
	}
}

func TestCache_Expiration(t *testing.T) {
	cache := NewCache()

	// Set a key with a short expiration
	cache.Set("short", "value", 10*time.Millisecond)

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Try to get the expired key
	_, found := cache.Get("short")
	if found {
		t.Error("Expected short to be expired")
	}
}

func TestCache_CleanupExpired(t *testing.T) {
	cache := NewCache()

	// Set some keys with different expiration times
	cache.Set("expired1", "value1", 10*time.Millisecond)
	cache.Set("expired2", "value2", 10*time.Millisecond)
	cache.Set("valid", "value3", 1*time.Minute)

	// Wait for some keys to expire
	time.Sleep(20 * time.Millisecond)

	// Run cleanup
	cache.CleanupExpired()

	// Check if expired keys were removed and valid key remains
	_, found1 := cache.Get("expired1")
	_, found2 := cache.Get("expired2")
	_, found3 := cache.Get("valid")

	if found1 || found2 {
		t.Error("Expected expired keys to be removed")
	}
	if !found3 {
		t.Error("Expected valid key to remain")
	}
}

func TestCache_Concurrency(t *testing.T) {
	cache := NewCache()
	var wg sync.WaitGroup
	concurrency := 100

	// Concurrent writes
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", i)
			cache.Set(key, i, 1*time.Minute)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", i)
			_, _ = cache.Get(key)
		}(i)
	}

	wg.Wait()

	// Verify all keys are present
	for i := 0; i < concurrency; i++ {
		key := fmt.Sprintf("key%d", i)
		_, found := cache.Get(key)
		if !found {
			t.Errorf("Expected to find key: %s", key)
		}
	}
}

func BenchmarkCache_Set(b *testing.B) {
	cache := NewCache()
	for i := 0; i < b.N; i++ {
		cache.Set(fmt.Sprintf("key%d", i), i, 1*time.Minute)
	}
}

func BenchmarkCache_Get(b *testing.B) {
	cache := NewCache()
	for i := 0; i < 1000; i++ {
		cache.Set(fmt.Sprintf("key%d", i), i, 1*time.Minute)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(fmt.Sprintf("key%d", i%1000))
	}
}

func BenchmarkCache_SetParallel(b *testing.B) {
	cache := NewCache()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Set(fmt.Sprintf("key%d", i), i, 1*time.Minute)
			i++
		}
	})
}

func BenchmarkCache_GetParallel(b *testing.B) {
	cache := NewCache()
	for i := 0; i < 1000; i++ {
		cache.Set(fmt.Sprintf("key%d", i), i, 1*time.Minute)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Get(fmt.Sprintf("key%d", i%1000))
			i++
		}
	})
}
