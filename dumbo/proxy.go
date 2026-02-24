package dumbo

import (
	"bytes"
	"crypto/tls"
	"encoding/pem"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"

	"golang.org/x/crypto/pkcs12"
)

// loggingReader wraps an io.ReadCloser to log data as it is read.
type loggingReader struct {
	rc io.ReadCloser
}

func (lr *loggingReader) Read(p []byte) (n int, err error) {
	n, err = lr.rc.Read(p)
	if n > 0 {
		slog.Debug("Response body chunk", "data", string(p[:n]))
	}
	return n, err
}

func (lr *loggingReader) Close() error {
	return lr.rc.Close()
}

// ReverseProxy is a wrapper around httputil.ReverseProxy that implements the http.Handler interface.
// It is designed to intercept local HTTP requests and forward them to a target HTTPS host
// using mutual TLS (mTLS) if configured.
//
// Usage:
//
//	proxy := NewReverseProxy(tlsConfig, true)
//	http.Handle("/", proxy)
//	http.ListenAndServe(":5000", nil)
type ReverseProxy struct {
	proxy *httputil.ReverseProxy
}

// NewReverseProxy initializes a ReverseProxy with a custom TLS configuration and optimized settings
// for streaming and SSE (Server-Sent Events).
//
// It configures the underlying transport to:
// - Use the provided tlsConfig (for mTLS).
// - Respect system proxy settings (HTTP_PROXY, HTTPS_PROXY).
// - Disable HTTP/2 to ensure stable streaming behavior in nested proxy environments.
// - Flush data immediately (FlushInterval: -1) to support real-time responses.
//
// If debug is true, it also logs the response status, headers, and body chunks.
func NewReverseProxy(tlsConfig *tls.Config, debug bool) *ReverseProxy {
	// The Director function modifies the incoming request to point to the target.
	director := func(req *http.Request) {
		// Dumbo uses the first segment of the path as the target host.
		// Path format: /{target_host}/{remote_path}
		path := strings.TrimPrefix(req.URL.Path, "/")
		parts := strings.SplitN(path, "/", 2)

		targetHost := parts[0]
		remainingPath := ""
		if len(parts) > 1 {
			remainingPath = "/" + parts[1]
		}

		// Re-write the request to use HTTPS and the target host/path.
		req.URL.Scheme = "https"
		req.URL.Host = targetHost
		req.URL.Path = remainingPath
		req.Host = targetHost // Crucial for SNI and Host header matching.

		slog.Debug("Director: forwarding request", "scheme", req.URL.Scheme, "host", req.URL.Host, "path", req.URL.Path)
	}

	transport := &http.Transport{
		TLSClientConfig:    tlsConfig,
		Proxy:              http.ProxyFromEnvironment,
		DisableCompression: true,
		// Explicitly disable HTTP/2. This forces HTTP/1.1 which is often more reliable
		// for SSE/streaming through multiple layers of proxies (like LiteLLM).
		TLSNextProto: make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	}

	modifyResponse := func(resp *http.Response) error {
		if debug {
			slog.Debug("Response received", "status", resp.Status, "headers", resp.Header)
			resp.Body = &loggingReader{rc: resp.Body}
		}
		return nil
	}

	return &ReverseProxy{
		proxy: &httputil.ReverseProxy{
			Director:       director,
			Transport:      transport,
			ModifyResponse: modifyResponse,
			FlushInterval:  -1, // Disables buffering. Each chunk from the server is sent to the client immediately.
			ErrorLog:       slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
		},
	}
}

// ServeHTTP satisfies the http.Handler interface, allowing ReverseProxy to be used by the standard library's HTTP server.
//
// Logic flow:
// 1. It checks if a target host is provided in the path.
// 2. If valid, it delegates the request handling to the internal httputil.ReverseProxy.
// 3. The internal proxy calls the Director (defined in NewReverseProxy) to transform the request.
// 4. The internal proxy then executes the request using the custom mTLS transport.
//
// This is typically called by net/http's server loop for every incoming request.
func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" || r.URL.Path == "" {
		http.Error(w, "Target host is required in the path (e.g., http://localhost:5000/google.com)", http.StatusBadRequest)
		return
	}

	// Log request headers and body
	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}
	slog.Debug("Incoming Request",
		"method", r.Method,
		"url", r.URL.String(),
		"headers", r.Header,
		"body", string(bodyBytes),
	)

	p.proxy.ServeHTTP(w, r)
}

// LoadPKCS12 loads and decodes a .p12 (PKCS#12) file into a tls.Config suitable for client authentication.
// It converts the P12 blocks into PEM format to create an X509 key pair.
func LoadPKCS12(path, password string) (*tls.Config, error) {
	p12Data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Extract PEM blocks from P12.
	blocks, err := pkcs12.ToPEM(p12Data, password)
	if err != nil {
		return nil, err
	}

	var pemData []byte
	for _, b := range blocks {
		pemData = append(pemData, pem.EncodeToMemory(b)...)
	}

	// Create the certificate pair.
	cert, err := tls.X509KeyPair(pemData, pemData)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}, nil
}
