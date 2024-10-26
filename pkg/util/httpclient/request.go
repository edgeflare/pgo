package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
)

// Request performs an HTTP request with exponential backoff retry logic.
//
// It constructs an HTTP request with the provided method, URL, optional payload,
// and headers. The request includes a timeout of 5 seconds and automatically sets
// "Content-Type: application/json" for POST, PUT, and PATCH methods if no
// Content-Type header is provided.
//
// If the request fails, it retries using an exponential backoff strategy
// with a context deadline. The function logs retries at the INFO level.
//
// On successful completion, the function returns the response body as a byte slice.
// Otherwise, it returns an error indicating the reason for the failure.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
//	defer cancel()
//
//	basicAuth := fmt.Sprintf("%s:%s", "username", "password")
//	basicAuthBase64 := base64.StdEncoding.EncodeToString([]byte(basicAuth))
//	headers := map[string][]string{
//		// "Authorization": {fmt.Sprintf("Bearer %s", os.Getenv("TOKEN"))},
//		"Authorization": {fmt.Sprintf("Basic %s", basicAuthBase64)},
//	}
//
//	body, err := httpclient.Request(ctx, http.MethodGet, "https://example.org", nil, headers)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Printf("Response: %s\n", body)
func Request(ctx context.Context, method, url string, payload []byte, headers map[string][]string, requestTimeout ...time.Duration) ([]byte, error) {
	var reqBody io.Reader
	if payload != nil {
		reqBody = bytes.NewReader(payload)
	}
	// if payload != nil {
	// 	payloadBytes, err := json.Marshal(payload)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to marshal request payload: %w", err)
	// 	}
	// 	reqBody = bytes.NewReader(payloadBytes)
	// }

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Set Content-Type only for methods that typically send payload in the body
	if reqBody != nil && (method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch) {
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	// Use custom timeout if provided, otherwise default to 5 seconds
	timeout := 5 * time.Second
	if len(requestTimeout) > 0 {
		timeout = requestTimeout[0]
		if timeout <= 0 {
			return nil, fmt.Errorf("invalid timeout duration")
		}
	}
	client := &http.Client{Timeout: timeout}

	var body []byte
	var firstAttempt = true // Track if the first attempt failed

	// Exponential backoff strategy
	bo := backoff.WithContext(backoff.NewExponentialBackOff(), ctx)
	err = backoff.Retry(func() error {
		// Check if this is a retry attempt (not the first one)
		if !firstAttempt {
			log.Printf("Retrying request to %s", url)
		}

		resp, err := client.Do(req)
		if err != nil {
			firstAttempt = false // Mark that the first attempt has failed
			return fmt.Errorf("failed to send HTTP request: %w", err)
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			firstAttempt = false // Mark that the first attempt has failed
			return fmt.Errorf("failed to read HTTP response body: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			firstAttempt = false // Mark that the first attempt has failed
			return fmt.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
		}

		// Success
		return nil
	}, bo)

	if err != nil {
		log.Printf("Request failed after retries: %v", err)
		return nil, err
	}

	return body, nil
}
