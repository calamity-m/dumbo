package dumbo

import (
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"golang.org/x/crypto/pkcs12"
)

type Proxy struct {
	Client *http.Client
	Scheme string
	Debug  bool
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse target host and path
	path := strings.TrimPrefix(r.URL.Path, "/")

	// Handle cases where the path might contain the scheme but only with one slash due to normalization
	if strings.HasPrefix(path, "http:/") && !strings.HasPrefix(path, "http://") {
		path = "http://" + path[6:]
	} else if strings.HasPrefix(path, "https:/") && !strings.HasPrefix(path, "https://") {
		path = "https://" + path[7:]
	}

	var targetURL *url.URL
	var err error

	if strings.Contains(path, "://") {
		targetURL, err = url.Parse(path)
	} else {
		targetURL, err = url.Parse(p.Scheme + "://" + path)
	}

	if err != nil || targetURL.Host == "" {
		slog.Error(fmt.Sprintf("Invalid target URL from path %q: %v", path, err))
		http.Error(w, "Invalid request format. Expected /{host}/{path}", http.StatusBadRequest)
		return
	}

	// Preserve query parameters
	targetURL.RawQuery = r.URL.RawQuery

	slog.Info(fmt.Sprintf("%s %s -> %s", r.Method, r.URL.String(), targetURL.String()))

	director := func(req *http.Request) {
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.URL.Path = targetURL.Path
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
		req.Host = targetURL.Host
		req.URL.RawQuery = targetURL.RawQuery

		if p.Debug {
			slog.Debug("Request Headers:")
			for name, values := range req.Header {
				for _, value := range values {
					slog.Debug(fmt.Sprintf("  %s: %s", name, value))
				}
			}
		}
	}

	proxy := &httputil.ReverseProxy{
		Director:      director,
		Transport:     p.Client.Transport,
		FlushInterval: -1, // Flush immediately for streaming/SSE
		ErrorLog:      slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
	}

	proxy.ServeHTTP(w, r)
}

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
