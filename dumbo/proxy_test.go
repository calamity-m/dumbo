package dumbo

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestStreamingProxy(t *testing.T) {
	// Create a mock target server that streams data
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		for i := 0; i < 5; i++ {
			fmt.Fprintf(w, "data: message %d\n\n", i)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer targetServer.Close()

	targetURL := strings.TrimPrefix(targetServer.URL, "http://")

	proxy := &Proxy{
		Client: &http.Client{},
		Scheme: "http",
	}

	// We use a real client against a server running the proxy.
	proxyServer := httptest.NewServer(proxy)
	defer proxyServer.Close()

	proxyPath := fmt.Sprintf("/%s/stream", targetURL)
	resp, err := http.Get(proxyServer.URL + proxyPath)
	if err != nil {
		t.Fatalf("Failed to call proxy: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	for i := 0; i < 5; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Failed to read from stream: %v", err)
		}
		expected := fmt.Sprintf("data: message %d", i)
		if !strings.Contains(line, expected) {
			t.Errorf("Expected message %d, got %v", i, line)
		}
		// Read the empty line
		_, _ = reader.ReadString('\n')
	}
}
