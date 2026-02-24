# Dumbo

Dumbo is a Go-based forward proxy designed to facilitate mutual TLS (mTLS) connections using password-encrypted `.p12` (PKCS#12) certificates. It acts as a bridge, allowing you to make simple HTTP calls locally that Dumbo then upgrades to mTLS-secured HTTPS calls to your target servers.

It is dumb, it is simple, I don't know why nginx and other things just don't take in
.p12(s) by default, seriously.

## Features

- **mTLS Support**: Easily use `.p12` certificates for client authentication.
- **SSE & Streaming**: Optimized for real-time data with immediate flushing and no buffering.
- **Environment Proxy Aware**: Automatically respects `HTTP_PROXY` and `HTTPS_PROXY` environment variables.
- **HTTP/1.1 Focused**: Forces HTTP/1.1 for maximum compatibility with corporate proxies and streaming endpoints.
- **Flexible CLI**: Options for custom CA certs, insecure modes, and running without mTLS.
- **Simple URL Mapping**: Map local paths directly to target hosts.

## How it Works

Dumbo operates as a transparent reverse proxy that translates local HTTP requests into authenticated mTLS HTTPS requests.

1. **URL Translation**: It takes the first segment of the incoming request path as the target host and the remainder as the remote path.
   - `http://localhost:5000/api.openai.com/v1/chat/completions` → `https://api.openai.com/v1/chat/completions`
2. **Mutual TLS**: If a `.p12` certificate is provided, Dumbo uses it to perform client authentication with the target server.
3. **Streaming/SSE**: Dumbo is configured with `FlushInterval: -1`, meaning it flushes data to the client as soon as it receives it from the server. This is critical for Server-Sent Events (SSE) used by LLM providers.
4. **Proxy Chaining**: It uses `http.ProxyFromEnvironment`, allowing Dumbo itself to run behind another corporate proxy if needed.
5. **Connection Handling**: It explicitly disables HTTP/2 for the upstream connection to ensure stable streaming behavior and avoid common protocol-level mismatches in nested proxy environments.

## Installation

Ensure you have [Go](https://go.dev/doc/install) installed.

```bash
# Clone the repository
git clone https://github.com/calamity-m/dumbo.git
cd dumbo

# Build the binary
go build -o build/dumbo main.go
```

## Usage

### Starting the Proxy

**With mTLS (Recommended):**
```bash
./build/dumbo --cert /path/to/identity.p12
```
*Dumbo will prompt you for the passphrase.*

**With a Custom CA Certificate:**
```bash
./build/dumbo --cert identity.p12 --cacert /path/to/server-ca.crt
```

**Running Insecurely (Skip Server Verification):**
```bash
./build/dumbo --cert identity.p12 --insecure
```

**Without mTLS:**
```bash
./build/dumbo --no-mtls
```

### Making Requests

Dumbo follows a simple URL pattern: `http://localhost:5000/{target_host}/{path}`.

**Example: GET request**
If you want to reach `https://api.internal.net/v1/users`, you would call:
```bash
curl "http://localhost:5000/api.internal.net/v1/users"
```

**Example: POST request with Body**
Dumbo forwards all methods and bodies:
```bash
curl -X POST "http://localhost:5000/api.internal.net/v1/data" 
     -H "Content-Type: application/json" 
     -d '{"key": "value"}'
```

**Example: Query Parameters**
```bash
curl "http://localhost:5000/api.internal.net/search?q=dumbo"
```

## CLI Options

| Flag | Description |
|------|-------------|
| `--cert` | Path to the .p12 certificate file (required unless --no-mtls is used) |
| `--cacert` | Path to the CA certificate file for server verification |
| `--port` | Port to listen on (default: 5000) |
| `--insecure` | Skip verification of the target server's certificate |
| `--no-mtls` | Run without mutual TLS (no .p12 required) |
| `--debug` | Enable debug logging (verbose output) |
| `--plain` | Disable pretty printing (colors, etc.) |
| `--help` | Show usage information |

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

