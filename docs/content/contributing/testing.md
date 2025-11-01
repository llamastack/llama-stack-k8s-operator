# Testing Guide

Testing guidelines for the LlamaStack Kubernetes Operator.

## Test Types

### Unit Tests

```bash
# Run unit tests
make test

# Run with coverage
make test-coverage

# Run specific package
go test ./controllers/...
```

### Integration Tests

```bash
# Run integration tests
make test-integration

# Run e2e tests
make test-e2e
```

## Writing Tests

### Controller Tests

```go
func TestLlamaStackDistributionController(t *testing.T) {
    // Test implementation
}
```

### E2E Tests

```go
func TestE2EDeployment(t *testing.T) {
    // E2E test implementation
}
```

## Next Steps

- [Development Guide](development.md)
- [Documentation Guide](documentation.md)
