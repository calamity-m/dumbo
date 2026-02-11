package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleProxy(t *testing.T) {
	// Create a mock target server
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/test/path" {
			t.Errorf("Expected path /test/path, got %s", r.URL.Path)
		}
		if r.URL.RawQuery != "a=b" {
			t.Errorf("Expected query a=b, got %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "success")
	}))
	defer targetServer.Close()

	// targetServer.URL will be something like http://127.0.0.1:12345
	targetURL := strings.TrimPrefix(targetServer.URL, "http://")

	// Create the proxy handler using the production Proxy struct
	proxy := &Proxy{
		Client: &http.Client{},
		Scheme: "http", // Use http for testing against httptest.Server
	}

	// Create a proxy request: /{host}/{path}?{query}
	proxyPath := fmt.Sprintf("/%s/test/path?a=b", targetURL)
	req := httptest.NewRequest("GET", proxyPath, nil)
	rr := httptest.NewRecorder()

	proxy.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", rr.Code)
	}
	if rr.Body.String() != "success" {
		t.Errorf("Expected body 'success', got %v", rr.Body.String())
	}
}
