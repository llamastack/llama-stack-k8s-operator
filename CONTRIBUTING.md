# Contributing to Llama Stack K8s Operator

Thank you for your interest in contributing to the Llama Stack K8s Operator! This document provides guidelines and instructions for contributing to this project.

## Development Setup

### Prerequisites

- Go (version specified in go.mod)
- Make
- pre-commit

### Pre-commit Hooks

This project uses pre-commit hooks to ensure code quality and consistency. The pre-commit hooks are automatically run on every commit and are also checked in our CI pipeline.

#### Setting up pre-commit

1. Install pre-commit by reference to the [pre-commit docs](https://pre-commit.com/#install)

2. Install the pre-commit hooks (optional):
   ```bash
   pre-commit install
   ```

#### Running Pre-commit Manually

You can run pre-commit hooks manually on all files:

```bash
pre-commit run --all-files
```

Or on specific files:

```bash
pre-commit run --files file1 file2
```

### CI Checks

The pre-commit hooks are also run in our CI pipeline on every pull request and push to the main branch. The CI will fail if:

1. Any pre-commit hooks fail
2. There are uncommitted changes after running pre-commit
3. There are new files that haven't been committed

To avoid CI failures, always run pre-commit locally before pushing your changes:

```bash
pre-commit run --all-files
git add .
git commit -m "Your commit message"
```

## Pull Request Process

1. Ensure your code passes all pre-commit checks locally
2. Create a pull request against the main branch
3. Ensure all CI checks pass
4. Wait for review and address any feedback

## Code Style

Please follow the project's code style guidelines. The pre-commit hooks will help enforce many of these automatically.

All error messages in the codebase must follow a consistent format to improve readability and maintainability. The pre-commit hook `check-go-error-messages` enforces these rules automatically.

### Rules for Error Messages

1. All error messages must start with "failed to"
2. Error messages should be descriptive and actionable

## Questions?

If you have any questions about contributing, please open an issue in the repository.
