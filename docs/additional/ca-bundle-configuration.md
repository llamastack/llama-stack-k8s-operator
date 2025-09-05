# CA Bundle Configuration for LlamaStackDistribution

This document explains how to configure custom CA bundles for LlamaStackDistribution to enable secure communication with external LLM providers using self-signed certificates.

## Overview

The CA bundle configuration allows you to:
- Use self-signed certificates for external LLM API connections
- Trust custom Certificate Authorities (CAs) for secure communication
- Provide CA certificates inline in the LlamaStackDistribution spec

## How It Works

When you configure a CA bundle:

1. **Inline Storage**: CA certificates are provided directly in the LlamaStackDistribution spec as PEM-encoded data
2. **Automatic ConfigMap Creation**: The operator automatically creates a ConfigMap containing the CA bundle data
3. **Volume Mounting**: The certificates are mounted at `/etc/ssl/certs/ca-bundle.crt` in the container
4. **Automatic Restarts**: Pods restart automatically when the CA bundle data changes

## Configuration

### Basic CA Bundle Configuration

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-llama-stack
spec:
  server:
    distribution:
      name: remote-vllm
    tlsConfig:
      caBundle: |
        -----BEGIN CERTIFICATE-----
        MIIDXTCCAkWgAwIBAgIJAKoK/heBjcOuMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
        BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
        aWRnaXRzIFB0eSBMdGQwHhcNMTMwODI3MjM1NDA3WhcNMjMwODI1MjM1NDA3WjBF
        MQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UECgwYSW50
        ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB
        CgKCAQEAwuqTiuGqAXTAM4PLnL6jrOMiTUps8gmI8DnJTtIQN9XgHk7ckY6+8X9s
        -----END CERTIFICATE-----
```

### Multiple CA Certificates

You can include multiple CA certificates in a single CA bundle:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-llama-stack
spec:
  server:
    distribution:
      name: remote-vllm
    tlsConfig:
      caBundle: |
        -----BEGIN CERTIFICATE-----
        # First CA certificate
        MIIDXTCCAkWgAwIBAgIJAKoK/heBjcOuMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
        # ... certificate data ...
        -----END CERTIFICATE-----
        -----BEGIN CERTIFICATE-----
        # Second CA certificate
        MIIDYTCCAkmgAwIBAgIJALfggjqwGI5jMA0GCSqGSIb3DQEBBQUAMEYxCzAJBgNV
        # ... certificate data ...
        -----END CERTIFICATE-----
```

## Examples

### Example 1: VLLM with Custom CA

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: secure-vllm-stack
spec:
  replicas: 1
  server:
    distribution:
      name: remote-vllm
    containerSpec:
      port: 8321
      env:
      - name: INFERENCE_MODEL
        value: "meta-llama/Llama-3.2-1B-Instruct"
      - name: VLLM_URL
        value: "https://vllm-server.vllm-dist.svc.cluster.local:8000/v1"
      - name: VLLM_TLS_VERIFY
        value: "/etc/ssl/certs/ca-bundle.crt"
    tlsConfig:
      caBundle: |
        -----BEGIN CERTIFICATE-----
        # Your VLLM server's CA certificate
        MIIDXTCCAkWgAwIBAgIJAKoK/heBjcOuMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
        # ... certificate data ...
        -----END CERTIFICATE-----
```

### Example 2: Ollama with Custom CA

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: secure-ollama-stack
spec:
  replicas: 1
  server:
    distribution:
      name: ollama
    containerSpec:
      port: 8321
      env:
      - name: INFERENCE_MODEL
        value: "llama3.2:1b"
      - name: OLLAMA_URL
        value: "https://ollama-server.ollama-dist.svc.cluster.local:11434"
    tlsConfig:
      caBundle: |
        -----BEGIN CERTIFICATE-----
        # Your Ollama server's CA certificate
        MIIDXTCCAkWgAwIBAgIJAKoK/heBjcOuMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
        # ... certificate data ...
        -----END CERTIFICATE-----
```

## Use Cases

### 1. Private Cloud Providers

When using private cloud LLM providers with self-signed certificates:

```yaml
spec:
  server:
    distribution:
      name: remote-vllm
    containerSpec:
      env:
      - name: VLLM_URL
        value: "https://private-llm-api.company.com/v1"
      - name: VLLM_TLS_VERIFY
        value: "/etc/ssl/certs/ca-bundle.crt"
    tlsConfig:
      caBundle: |
        -----BEGIN CERTIFICATE-----
        # Private cloud provider's CA certificate
        # ... certificate data ...
        -----END CERTIFICATE-----
```

### 2. Internal Enterprise APIs

For enterprise environments with internal CAs:

```yaml
spec:
  server:
    distribution:
      name: remote-vllm
    tlsConfig:
      caBundle: |
        -----BEGIN CERTIFICATE-----
        # Enterprise root CA certificate
        # ... certificate data ...
        -----END CERTIFICATE-----
        -----BEGIN CERTIFICATE-----
        # Enterprise intermediate CA certificate
        # ... certificate data ...
        -----END CERTIFICATE-----
```

### 3. Development/Testing

For development environments with self-signed certificates:

```yaml
spec:
  server:
    distribution:
      name: ollama
    tlsConfig:
      caBundle: |
        -----BEGIN CERTIFICATE-----
        # Development self-signed CA certificate
        # ... certificate data ...
        -----END CERTIFICATE-----
```

## Obtaining CA Certificates

### From a Server

```bash
# Get the CA certificate from a server
openssl s_client -showcerts -connect your-server.com:443 </dev/null 2>/dev/null | \
  openssl x509 -outform PEM > ca-certificate.pem
```

### From a Certificate File

```bash
# Extract CA certificate from a certificate bundle
openssl x509 -in certificate-bundle.pem -out ca-certificate.pem
```

### From Kubernetes Secrets

```bash
# Extract CA certificate from a Kubernetes secret
kubectl get secret your-tls-secret -o jsonpath='{.data.ca\.crt}' | base64 -d > ca-certificate.pem
```

## Troubleshooting

### Common Issues

1. **Invalid Certificate Format**: Ensure the certificate is in PEM format with proper BEGIN/END blocks
2. **Certificate Validation**: Verify the certificate is valid and not expired
3. **Pod Not Restarting**: The operator automatically restarts pods when CA bundle data changes

### Common Error Messages and Solutions

#### "CA bundle contains invalid PEM data"
- **Cause**: The certificate data is not in valid PEM format
- **Solution**: Ensure certificates are properly formatted with BEGIN/END CERTIFICATE blocks
- **Example**: Valid format starts with `-----BEGIN CERTIFICATE-----`

#### "Failed to parse certificate"
- **Cause**: Certificate data is corrupted or not a valid X.509 certificate
- **Solution**: Regenerate the certificate or verify the source
- **Example**: Use `openssl x509 -text -noout -in your-cert.pem` to validate certificates

### Debugging

```bash
# Check the automatically created ConfigMap
kubectl get configmap <llamastack-name>-config -o yaml

# Check pod environment variables
kubectl describe pod <llama-stack-pod-name>

# Check mounted certificates
kubectl exec <llama-stack-pod-name> -- ls -la /etc/ssl/certs/

# Validate certificate format locally
openssl x509 -text -noout -in ca-certificate.pem

# Check certificate expiration
openssl x509 -enddate -noout -in ca-certificate.pem

# Test certificate chain
openssl verify -CAfile ca-certificate.pem server.crt
```

### Validation Checklist

Before deploying a LlamaStackDistribution with CA bundle:

- [ ] Certificate data is in PEM format
- [ ] Certificate data contains valid X.509 certificates
- [ ] Certificate is not expired
- [ ] Certificate contains the expected CA for your external service
- [ ] Certificate data is properly indented in the YAML (use `|` for multiline strings)

## Security Considerations

1. **Sensitive Data**: CA certificates are stored in Kubernetes ConfigMaps created by the operator
2. **RBAC**: Ensure appropriate RBAC policies are in place for accessing the LlamaStackDistribution resources
3. **Certificate Rotation**: Update the CA bundle in the LlamaStackDistribution spec when certificates are rotated
4. **Validation**: The operator validates PEM format but doesn't verify certificate validity or expiration
