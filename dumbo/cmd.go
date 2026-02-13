package dumbo

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	certPath    string
	caCertPath  string
	port        int
	insecure    bool
	noMTLS      bool
	logLevelStr string
	plain       bool
)

// NewAnonymousClient creates a client optimized for corporate proxies.
// It forces HTTP/1.1, mimics curl, and strictly ignores proxy auth.
func NewAnonymousClient(tlsConfig *tls.Config) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
			// 1. Force HTTP/1.1
			// Setting TLSNextProto to a non-nil empty map disables HTTP/2.
			TLSNextProto: make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),

			// 2. Custom Proxy Logic
			Proxy: func(req *http.Request) (*url.URL, error) {
				// Get the proxy from the environment (HTTPS_PROXY, etc.)
				proxyURL, err := http.ProxyFromEnvironment(req)
				if proxyURL == nil || err != nil {
					return proxyURL, err
				}

				// 3. Strip Credentials intentionally
				// This ensures we never send a Proxy-Authorization header
				proxyURL.User = nil
				return proxyURL, nil
			},
		},
	}
}

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dumbo",
		Short: "Dumbo is a Go-based forward proxy for mTLS connections",
		Long:  `Dumbo is a Go-based forward proxy designed to facilitate mutual TLS (mTLS) connections using password-encrypted .p12 certificates.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			level := ParseLevel(logLevelStr)
			logger := slog.New(NewInterceptHandler(os.Stdout, nil, !plain, level))
			slog.SetDefault(logger)

			if certPath == "" && !noMTLS {
				return fmt.Errorf("--cert flag is required unless --no-mtls is specified")
			}

			var tlsConfig *tls.Config
			var err error

			if !noMTLS {
				fmt.Printf("Enter passphrase for %s: ", certPath)
				var passphraseBytes []byte
				passphraseBytes, err = term.ReadPassword(int(syscall.Stdin))
				if err != nil {
					return fmt.Errorf("\nError reading passphrase: %w", err)
				}
				fmt.Println()

				tlsConfig, err = LoadPKCS12(certPath, string(passphraseBytes))
				if err != nil {
					return fmt.Errorf("Error loading PKCS12: %w", err)
				}
			} else {
				tlsConfig = &tls.Config{}
			}

			if caCertPath != "" {
				caCert, err := os.ReadFile(caCertPath)
				if err != nil {
					return fmt.Errorf("Error reading CA certificate: %w", err)
				}
				caCertPool := x509.NewCertPool()
				if !caCertPool.AppendCertsFromPEM(caCert) {
					return fmt.Errorf("Error appending CA certificate to pool")
				}
				tlsConfig.RootCAs = caCertPool
			}

			if insecure {
				tlsConfig.InsecureSkipVerify = true
			}

			client := NewAnonymousClient(tlsConfig)

			addr := fmt.Sprintf(":%d", port)
			slog.Info(fmt.Sprintf("Dumbo proxy listening on http://localhost%s", addr))

			proxy := &Proxy{
				Client: client,
				Scheme: "https",
				Debug:  level <= slog.LevelDebug,
			}

			http.HandleFunc("/", proxy.ServeHTTP)

			return http.ListenAndServe(addr, nil)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&certPath, "cert", "", "Path to the .p12 certificate file")
	flags.StringVar(&caCertPath, "cacert", "", "Path to the CA certificate file for server verification")
	flags.IntVar(&port, "port", 5000, "Port to listen on")
	flags.BoolVar(&insecure, "insecure", false, "Skip verification of the target server's certificate")
	flags.BoolVar(&noMTLS, "no-mtls", false, "Run without mutual TLS (no .p12 required)")
	flags.StringVar(&logLevelStr, "log-level", "info", "Log level (debug, info, warn, error)")
	flags.BoolVar(&plain, "plain", false, "Disable pretty printing (colors, etc.)")

	return cmd
}

func Execute() error {
	return NewRootCmd().Execute()
}
