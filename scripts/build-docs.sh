#!/bin/bash
set -e

# Build script for LlamaStack Operator Documentation
# This script builds the documentation site locally for development and testing

echo "🚀 Building LlamaStack Operator Documentation..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if we're in the right directory
if [ ! -f "Makefile" ] || [ ! -d "docs" ]; then
    print_error "This script must be run from the repository root directory"
    exit 1
fi

# Check dependencies
print_status "Checking dependencies..."

# Check Go
if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go 1.21 or later."
    exit 1
fi

GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
REQUIRED_GO_VERSION="1.21"

if [ "$(printf '%s\n' "$REQUIRED_GO_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_GO_VERSION" ]; then
    print_error "Go version $REQUIRED_GO_VERSION or later is required. Found: $GO_VERSION"
    exit 1
fi

print_success "Go version $GO_VERSION found"

# Check Python
if ! command -v python3 &> /dev/null; then
    print_error "Python 3 is not installed. Please install Python 3.8 or later."
    exit 1
fi

PYTHON_VERSION=$(python3 --version | grep -oE '[0-9]+\.[0-9]+')
REQUIRED_PYTHON_VERSION="3.8"

if [ "$(printf '%s\n' "$REQUIRED_PYTHON_VERSION" "$PYTHON_VERSION" | sort -V | head -n1)" != "$REQUIRED_PYTHON_VERSION" ]; then
    print_error "Python version $REQUIRED_PYTHON_VERSION or later is required. Found: $PYTHON_VERSION"
    exit 1
fi

print_success "Python version $PYTHON_VERSION found"

# Check pip
if ! command -v pip3 &> /dev/null; then
    print_error "pip3 is not installed. Please install pip3."
    exit 1
fi

print_success "pip3 found"

# Install Go tools
print_status "Installing Go documentation tools..."

if ! make crd-ref-docs &> /dev/null; then
    print_error "Failed to install crd-ref-docs"
    exit 1
fi

if ! make gen-crd-api-reference-docs &> /dev/null; then
    print_warning "gen-crd-api-reference-docs installation failed, continuing with crd-ref-docs only"
fi

print_success "Go tools installed"

# Install Python dependencies
print_status "Installing Python dependencies..."

if [ -f "docs/requirements.txt" ]; then
    if ! pip3 install -r docs/requirements.txt; then
        print_error "Failed to install Python dependencies"
        exit 1
    fi
    print_success "Python dependencies installed"
else
    print_error "docs/requirements.txt not found"
    exit 1
fi

# Generate API documentation
print_status "Generating API documentation..."

if ! make api-docs; then
    print_error "Failed to generate API documentation"
    exit 1
fi

print_success "API documentation generated"

# Build documentation site
print_status "Building documentation site..."

if ! make docs-build; then
    print_error "Failed to build documentation site"
    exit 1
fi

print_success "Documentation site built successfully"

# Check if site directory exists and has content
if [ ! -d "docs/site" ]; then
    print_error "Documentation site directory not found"
    exit 1
fi

SITE_SIZE=$(du -sh docs/site | cut -f1)
print_success "Documentation site built (${SITE_SIZE})"

# Validate the build
print_status "Validating build..."

# Check for index.html
if [ ! -f "docs/site/index.html" ]; then
    print_error "index.html not found in build output"
    exit 1
fi

# Check for API documentation
if [ ! -f "docs/site/reference/api/index.html" ]; then
    print_warning "API reference page not found, but continuing..."
fi

# Check for assets
if [ ! -d "docs/site/assets" ]; then
    print_warning "Assets directory not found, but continuing..."
fi

print_success "Build validation completed"

# Display build information
echo ""
echo "📊 Build Summary:"
echo "=================="
echo "📁 Output directory: docs/site/"
echo "📏 Site size: ${SITE_SIZE}"
echo "🔗 Local preview: http://localhost:8000"
echo ""

# Offer to serve the site locally
read -p "🌐 Would you like to serve the documentation locally? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    print_status "Starting local server..."
    echo "📖 Documentation will be available at: http://localhost:8000"
    echo "🛑 Press Ctrl+C to stop the server"
    echo ""
    
    cd docs && python3 -m mkdocs serve --dev-addr 0.0.0.0:8000
else
    echo ""
    print_success "Documentation build completed successfully!"
    echo ""
    echo "To serve the documentation locally, run:"
    echo "  cd docs && mkdocs serve"
    echo ""
    echo "Or use the Makefile target:"
    echo "  make docs-serve"
    echo ""
fi
