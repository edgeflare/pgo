package httputil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestNewRouter tests the creation of a new Router
func TestNewRouter(t *testing.T) {
	r := NewRouter()
	if r == nil {
		t.Fatal("expected router to be non-nil")
	}
}

// TestRouterHandle tests route registration and handling
func TestRouterHandle(t *testing.T) {
	r := NewRouter()
	r.Handle("GET /test", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.mux.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", w.Result().StatusCode)
	}
}

// TestRouterMiddleware tests adding and using middleware
func TestRouterMiddleware(t *testing.T) {
	r := NewRouter()

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Test", "true")
			next.ServeHTTP(w, req)
		})
	})

	r.Handle("GET /test", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.mux.ServeHTTP(w, req)

	if w.Header().Get("X-Test") != "true" {
		t.Errorf("expected X-Test header to be set")
	}
}

// TestRouterGroup tests sub-router grouping
func TestRouterGroup(t *testing.T) {
	r := NewRouter()
	api := r.Group("/api")
	api.Handle("GET /v1/test", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	w := httptest.NewRecorder()
	r.mux.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", w.Result().StatusCode)
	}
}

// TestRouterListenAndServe tests server start and shutdown
func TestRouterListenAndServe(t *testing.T) {
	r := NewRouter()
	r.Handle("GET /test", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	serverAddr := ":8081"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := r.ListenAndServe(serverAddr); err != http.ErrServerClosed {
			t.Logf("expected server to close, got %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond) // Give the server a moment to start

	req, err := http.NewRequest("GET", "http://localhost:8081/test", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.StatusCode)
	}

	if err := r.Shutdown(ctx); err != nil {
		t.Fatalf("failed to shutdown server: %v", err)
	}

	wg.Wait()
}

// TestRouterTLS tests the TLS configuration of the router.
// func TestRouterTLS(t *testing.T) {
// 	certPath := "./test_tls/tls.crt"
// 	keyPath := "./test_tls/tls.key"
// 	_, err := util.LoadOrGenerateCert(certPath, keyPath)
// 	require.NoError(t, err)
// 	defer func() {
// 		os.Remove(certPath)
// 		os.Remove(keyPath)
// 		os.RemoveAll(filepath.Dir(certPath))
// 	}()

// 	r := NewRouter(WithTLS(certPath, keyPath))

// 	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusOK)
// 	})

// 	r.Handle("/test", handler)

// 	// Spin up a real HTTP server for TLS testing
// 	go func() {
// 		err := r.ListenAndServe(":8443")
// 		require.NoError(t, err)
// 	}()

// 	// Test HTTPS request
// 	req := httptest.NewRequest("GET", "https://localhost:8443/test", nil)
// 	req.URL.Scheme = "https"
// 	rec := httptest.NewRecorder()
// 	http.DefaultTransport.RoundTrip(req) // Use actual transport to avoid the `httptest` limitations

// 	res := rec.Result()
// 	require.Equal(t, http.StatusOK, res.StatusCode)
// }

// func TestListenAndServe_HTTPS(t *testing.T) {
// 	// Generate self-signed certificate for testing
// 	tempDir := t.TempDir()
// 	certFile := filepath.Join(tempDir, "tls.crt")
// 	keyFile := filepath.Join(tempDir, "tls.key")
// 	_, err := util.LoadOrGenerateCert(certFile, keyFile)
// 	require.NoError(t, err)
// 	defer func() {
// 		os.Remove(certFile)
// 		os.Remove(keyFile)
// 	}()

// 	r := NewRouter(WithTLS(certFile, keyFile))
// 	r.Handle("GET /test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		fmt.Fprint(w, "Hello, secure world!")
// 	}))

// 	// Start HTTPS server
// 	go func() {
// 		if err := r.ListenAndServe(":0"); err != nil && err != http.ErrServerClosed {
// 			t.Errorf("ListenAndServe() error = %v", err)
// 		}
// 	}()

// 	// Wait for the server to start
// 	time.Sleep(100 * time.Millisecond)

// 	// Make HTTPS request (ignoring certificate errors for self-signed cert)
// 	tr := &http.Transport{
// 		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Insecure for testing only
// 	}
// 	client := &http.Client{Transport: tr}
// 	resp, err := client.Get("https://" + r.server.Addr + "/test")
// 	require.NoError(t, err, "Failed to make HTTPS request")
// 	defer resp.Body.Close()

// 	body, err := io.ReadAll(resp.Body)
// 	require.NoError(t, err, "Failed to read response body")

// 	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected status code 200")
// 	assert.Equal(t, "Hello, secure world!", string(body), "Expected response body 'Hello, secure world!'")

// 	// Test Shutdown
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()
// 	err = r.Shutdown(ctx)
// 	assert.NoError(t, err, "Expected graceful shutdown")
// }

// BenchmarkRouterHandle benchmarks route registration
func BenchmarkRouterHandle(b *testing.B) {
	r := NewRouter()
	for i := 0; i < b.N; i++ {
		r.Handle("GET /test"+fmt.Sprintf("%d", i), http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	}
}

// BenchmarkRouterServeHTTP benchmarks serving HTTP requests
func BenchmarkRouterServeHTTP(b *testing.B) {
	r := NewRouter()
	r.Handle("GET /test", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.mux.ServeHTTP(w, req)
	}
}

// BenchmarkRouterHandleWithMiddleware benchmarks route registration with middleware
func BenchmarkRouterHandleWithMiddleware(b *testing.B) {
	r := NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// Example middleware
			next.ServeHTTP(w, req)
		})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Handle("GET /test"+fmt.Sprintf("%d", i), http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	}
}

// BenchmarkRouterServeHTTPConcurrent benchmarks serving HTTP requests concurrently
func BenchmarkRouterServeHTTPConcurrent(b *testing.B) {
	r := NewRouter()
	r.Handle("GET /test", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r.mux.ServeHTTP(w, req)
		}
	})
}
