package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"

	"golang.org/x/crypto/pkcs12"
	"golang.org/x/term"
)

func main() {
	certPath := flag.String("cert", "", "Path to the .p12 certificate file")
	caCertPath := flag.String("cacert", "", "Path to the CA certificate file for server verification")
	port := flag.Int("port", 5000, "Port to listen on")
	insecure := flag.Bool("insecure", false, "Skip verification of the target server's certificate")
	noMTLS := flag.Bool("no-mtls", false, "Run without mutual TLS (no .p12 required)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: dumbo --cert /path/to/my/p12 [options] or dumbo --no-mtls [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *certPath == "" && !*noMTLS {
		fmt.Fprintf(os.Stderr, "Error: --cert flag is required unless --no-mtls is specified\n")
		flag.Usage()
		os.Exit(1)
	}

	var tlsConfig *tls.Config
	var err error

	if !*noMTLS {
		fmt.Printf("Enter passphrase for %s: ", *certPath)
		var passphraseBytes []byte
		passphraseBytes, err = term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			log.Fatalf("\nError reading passphrase: %v", err)
		}
		fmt.Println()

		tlsConfig, err = loadPKCS12(*certPath, string(passphraseBytes))
		if err != nil {
			log.Fatalf("Error loading PKCS12: %v", err)
		}
	} else {
		tlsConfig = &tls.Config{}
	}

	if *caCertPath != "" {
		caCert, err := os.ReadFile(*caCertPath)
		if err != nil {
			log.Fatalf("Error reading CA certificate: %v", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			log.Fatalf("Error appending CA certificate to pool")
		}
		tlsConfig.RootCAs = caCertPool
	}

	if *insecure {
		tlsConfig.InsecureSkipVerify = true
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	client := &http.Client{Transport: transport}

	addr := fmt.Sprintf(":%d", *port)
	fmt.Printf("Dumbo proxy listening on http://localhost%s\n", addr)

	proxy := &Proxy{
		Client: client,
		Scheme: "https",
	}

	http.HandleFunc("/", proxy.ServeHTTP)

	log.Fatal(http.ListenAndServe(addr, nil))
}

type Proxy struct {
	Client *http.Client
	Scheme string
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handleProxy(w, r, p.Client, p.Scheme)
}

func loadPKCS12(path, password string) (*tls.Config, error) {
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

func handleProxy(w http.ResponseWriter, r *http.Request, client *http.Client, scheme string) {
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
		Scheme:   scheme,
		Host:     targetHost,
		Path:     targetPath,
		RawQuery: r.URL.RawQuery,
	}

	fmt.Printf("Forwarding to: %s\n", targetURL.String())

	proxyReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to forward request: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}