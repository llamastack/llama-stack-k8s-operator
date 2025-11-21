# Documentation Guide

Guide for contributing to the LlamaStack Kubernetes Operator documentation.

## Documentation Structure

The documentation is built with MkDocs and follows this structure:

```
docs/
├── content/
│   ├── index.md
│   ├── getting-started/
│   ├── how-to/
│   ├── reference/
│   ├── examples/
│   └── contributing/
└── mkdocs.yml
```

## Writing Documentation

### Markdown Guidelines

- Use clear, concise language
- Include code examples
- Add diagrams where helpful
- Follow the existing style

### Code Examples

```yaml
# Always include complete, working examples
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: example
spec:
  image: llamastack/llamastack:latest
```

## Building Documentation

### Local Development

```bash
# Install dependencies
pip install -r docs/requirements.txt

# Serve locally
make docs-serve

# Build static site
make docs-build
```

### API Documentation

API documentation is auto-generated from Go types:

```bash
# Generate API docs
make api-docs
```

## Contributing

1. Edit markdown files in `docs/content/`
2. Test locally with `make docs-serve`
3. Submit a pull request

## Next Steps

- [Development Guide](development.md)
- [Testing Guide](testing.md)
