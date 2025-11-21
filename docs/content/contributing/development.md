# Development Guide

Guide for contributing to the LlamaStack Kubernetes Operator.

## Development Setup

### Prerequisites

- Go 1.24+
- Docker
- Kubernetes cluster (kind/minikube for local development)
- kubectl
- make

### Local Development

```bash
# Clone the repository
git clone https://github.com/llamastack/llama-stack-k8s-operator.git
cd llama-stack-k8s-operator

# Install dependencies
make deps

# Run tests
make test

# Build operator
make build

# Run locally
make run
```

## Contributing

### Code Style

- Follow Go conventions
- Use `gofmt` for formatting
- Add tests for new features
- Update documentation

### Pull Request Process

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Update documentation
6. Submit a pull request

## Next Steps

- [Testing Guide](testing.md)
- [Documentation Guide](documentation.md)
