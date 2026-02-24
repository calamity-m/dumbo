package dumbo

import (
	"crypto/tls"
	"encoding/pem"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"

	"golang.org/x/crypto/pkcs12"
)

// ReverseProxy handles the proxying logic.
type ReverseProxy struct {
	proxy *httputil.ReverseProxy
}

// NewReverseProxy creates a new reverse proxy with the given TLS configuration.
func NewReverseProxy(tlsConfig *tls.Config) *ReverseProxy {
	director := func(req *http.Request) {
		path := strings.TrimPrefix(req.URL.Path, "/")
		parts := strings.SplitN(path, "/", 2)
		
		targetHost := parts[0]
		remainingPath := ""
		if len(parts) > 1 {
			remainingPath = "/" + parts[1]
		}

		req.URL.Scheme = "https"
		req.URL.Host = targetHost
		req.URL.Path = remainingPath
		req.Host = targetHost
		
		slog.Debug("Director", "scheme", req.URL.Scheme, "host", req.URL.Host, "path", req.URL.Path)
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
		// Disable HTTP/2 by setting TLSNextProto to a non-nil empty map.
		TLSNextProto: make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	}

	return &ReverseProxy{
		proxy: &httputil.ReverseProxy{
			Director:      director,
			Transport:     transport,
			FlushInterval: -1, // Disable buffering for streaming
			ErrorLog:      slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
		},
	}
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" || r.URL.Path == "" {
		http.Error(w, "Target host is required in the path (e.g., /google.com)", http.StatusBadRequest)
		return
	}
	p.proxy.ServeHTTP(w, r)
}

// LoadPKCS12 loads a PKCS12 (.p12) certificate file.
func LoadPKCS12(path, password string) (*tls.Config, error) {
	p12Data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	blocks, err := pkcs12.ToPEM(p12Data, password)
	if err != nil {
		return nil, err
	}

	var pemData []byte
	for _, b := range blocks {
		pemData = append(pemData, pem.EncodeToMemory(b)...)
	}

	cert, err := tls.X509KeyPair(pemData, pemData)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}, nil
}
