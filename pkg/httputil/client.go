package httputil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
)

// RequestConfig holds configuration for HTTP requests
type RequestConfig struct {
	Logger          Logger
	Headers         map[string][]string
	ResponseHandler func(*http.Response) error
	Method          string
	URL             string
	Timeout         time.Duration
	MaxRetries      int
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	RetryEnabled    bool
}

// Logger interface for customizable logging
type Logger interface {
	Printf(format string, v ...interface{})
}

// DefaultRequestConfig returns a RequestConfig with sensible defaults
func DefaultRequestConfig(method, url string) RequestConfig {
	return RequestConfig{
		Method:         method,
		URL:            url,
		Timeout:        5 * time.Second,
		RetryEnabled:   true,
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		Logger:         log.Default(),
	}
}

// Response represents an HTTP response with additional metadata
type Response struct {
	Headers    http.Header
	Request    *http.Request
	Body       []byte
	StatusCode int
}

// Request performs an HTTP request with configurable retry logic
func Request(ctx context.Context, config RequestConfig, payload interface{}) (*Response, error) {
	var reqBody io.Reader
	if payload != nil {
		var payloadBytes []byte
		var err error

		switch v := payload.(type) {
		case []byte:
			payloadBytes = v
		case string:
			payloadBytes = []byte(v)
		default:
			payloadBytes, err = json.Marshal(payload)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal payload: %w", err)
			}
		}
		reqBody = bytes.NewReader(payloadBytes)
	}

	req, err := http.NewRequestWithContext(ctx, config.Method, config.URL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, values := range config.Headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Set default content-type for methods with body
	if reqBody != nil && (config.Method == http.MethodPost || config.Method == http.MethodPut || config.Method == http.MethodPatch) {
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	client := &http.Client{
		Timeout: config.Timeout,
	}

	var response *Response
	var firstAttempt = true

	operation := func() error {
		if !firstAttempt && config.Logger != nil {
			config.Logger.Printf("Retrying request to %s", config.URL)
		}

		resp, opErr := client.Do(req)
		if opErr != nil {
			firstAttempt = false
			return fmt.Errorf("request failed: %w", opErr)
		}
		defer resp.Body.Close()

		// Read response body
		body, opErr := io.ReadAll(resp.Body)
		if opErr != nil {
			return fmt.Errorf("failed to read response body: %w", opErr)
		}

		response = &Response{
			StatusCode: resp.StatusCode,
			Body:       body,
			Headers:    resp.Header,
			Request:    req,
		}

		// Custom response handling if provided
		if config.ResponseHandler != nil {
			if err = config.ResponseHandler(resp); err != nil { // Reuse outer 'err' variable
				firstAttempt = false
				return err
			}
		}

		// Default status code check
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			firstAttempt = false
			return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, body)
		}

		return nil
	}

	if config.RetryEnabled {
		// Configure backoff
		b := backoff.NewExponentialBackOff()
		b.InitialInterval = config.InitialBackoff
		b.MaxInterval = config.MaxBackoff
		b.MaxElapsedTime = time.Duration(config.MaxRetries) * config.MaxBackoff

		err = backoff.Retry(operation, backoff.WithContext(b, ctx)) // Reuse outer 'err' variable
	} else {
		err = operation() // Reuse outer 'err' variable
	}

	if err != nil {
		if config.Logger != nil {
			config.Logger.Printf("Request failed: %v", err)
		}
		return response, err // Return response even on error for inspection
	}

	return response, nil
}
