package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// Options holds the options for the proxy server
type Options struct {
	TLSConfig     *tls.Config
	TrimPrefix    string
	ForwardedHost string
}

// Serve creates a reverse proxy handler based on the given target and options
func Proxy(target string, opts Options) http.HandlerFunc {
	// Parse the target URL
	targetURL, err := url.Parse(target)
	if err != nil {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "invalid target URL", http.StatusInternalServerError)
		}
	}

	// Set default options
	if opts.ForwardedHost == "" {
		opts.ForwardedHost = targetURL.Host
	}
	if opts.TLSConfig == nil {
		opts.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         targetURL.Hostname(),
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Configure the Director function
	proxy.Director = func(req *http.Request) {
		// Preserve the original request context
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host

		// Update the Host header to the target host
		req.Host = targetURL.Host

		// Trim prefix if provided
		if opts.TrimPrefix != "" {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, opts.TrimPrefix)
		}

		// Set the X-Forwarded-Host header if provided
		if opts.ForwardedHost != "" {
			req.Header.Set("X-Forwarded-Host", opts.ForwardedHost)
		}

	}

	// Configure the proxy transport
	proxy.Transport = &http.Transport{
		TLSClientConfig: opts.TLSConfig,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	}
}
