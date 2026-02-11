package dumbo

import (
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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
	handleProxy(w, r, p)
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

func handleProxy(w http.ResponseWriter, r *http.Request, p *Proxy) {
	pathParts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
	if len(pathParts) < 1 || pathParts[0] == "" {
		http.Error(w, "Invalid request format. Expected /{host}/{path}", http.StatusBadRequest)
		return
	}

	targetHost := pathParts[0]
	targetPath := "/"
	if len(pathParts) > 1 {
		targetPath = "/" + pathParts[1]
	}

	targetURL := &url.URL{
		Scheme:   p.Scheme,
		Host:     targetHost,
		Path:     targetPath,
		RawQuery: r.URL.RawQuery,
	}

	slog.Info(fmt.Sprintf("%s %s -> %s", r.Method, r.URL.String(), targetURL.String()))

	if p.Debug {
		slog.Debug("Request Headers:")
		for name, values := range r.Header {
			for _, value := range values {
				slog.Debug(fmt.Sprintf("  %s: %s", name, value))
			}
		}
	}

	proxyReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to create request: %v", err))
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	resp, err := p.Client.Do(proxyReq)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to forward request: %v", err))
		http.Error(w, fmt.Sprintf("Failed to forward request: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	slog.Info(fmt.Sprintf("%s %s -> %d %s", r.Method, targetURL.String(), resp.StatusCode, http.StatusText(resp.StatusCode)))

	if p.Debug {
		slog.Debug("Response Headers:")
		for name, values := range resp.Header {
			for _, value := range values {
				slog.Debug(fmt.Sprintf("  %s: %s", name, value))
			}
		}
	}

	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
