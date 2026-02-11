# Dumbo

Dumbo is a forward proxy written in Go that provides mutual TLS (mTLS) using password-encrypted `.p12` certificates.

## Core Requirements

1. **Proxy Logic**: The proxy receives requests in the format `http://localhost:5000/{target_host}/{path}?{query}`.
2. **Forwarding**: It forwards the request to `https://{target_host}/{path}?{query}` using the same HTTP method and body.
3. **mTLS**: It uses a `.p12` certificate for mTLS.
4. **Security**: The proxy must prompt the user for the `.p12` passphrase at startup.
5. **CLI**:
    - `dumbo --cert /path/to/my/p12` to start the proxy.
    - `dumbo --cacert /path/to/my/ca.crt` to specify a CA certificate for server verification.
    - `dumbo --no-mtls` to run without mutual TLS (no .p12 required).
    - `dumbo --insecure` to skip verification of the target server's certificate.
    - `dumbo --help` for usage information.

## Implementation Details

- **Language**: Go
- **Certificate Format**: PKCS#12 (.p12)
- **Port**: Defaulting to 5000 (as per the example).
