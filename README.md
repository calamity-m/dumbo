# Dumbo

Dumbo is a Go-based forward proxy designed to facilitate mutual TLS (mTLS) connections using password-encrypted `.p12` (PKCS#12) certificates. It acts as a bridge, allowing you to make simple HTTP calls locally that Dumbo then upgrades to mTLS-secured HTTPS calls to your target servers.

It is dumb, it is simple, I don't know why nginx and other things just don't take in
.p12(s) by default, seriously.

## Features

- **mTLS Support**: Easily use `.p12` certificates for client authentication.
- **Secure Passphrase Entry**: Prompts for the `.p12` passphrase securely at startup.
- **Flexible CLI**: Options for custom CA certs, insecure modes, and running without mTLS.
- **Simple URL Mapping**: Map local paths directly to target hosts.

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
| `--log-level` | Log level (debug, info, warn, error) (default: info) |
| `--plain` | Disable pretty printing (colors, etc.) |
| `--help` | Show usage information |

## Testing

To run all tests in the project, use:

```bash
go test ./...
```

