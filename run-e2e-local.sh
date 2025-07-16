#!/usr/bin/env bash

set -e

# Default values
CLUSTER_NAME="llama-stack-e2e"
REGISTRY_NAME="kind-registry"
REGISTRY_PORT="5000"
IMAGE_TAG="local-test"
CLEANUP_ON_EXIT=true
SKIP_BUILD=false
VERBOSE=false

# Simple output functions
print_step() {
    echo "=== $1 ==="
}

print_success() {
    echo "✓ $1"
}

print_warning() {
    echo "⚠ $1"
}

print_error() {
    echo "✗ $1"
}

# Function to print usage
usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Run LlamaStack Kubernetes Operator E2E tests locally using Kind

OPTIONS:
    -c, --cluster-name NAME     Kind cluster name (default: llama-stack-e2e)
    -t, --tag TAG              Docker image tag (default: local-test)
    --skip-build               Skip building the operator image
    --no-cleanup               Don't cleanup cluster on exit
    -v, --verbose              Enable verbose output
    -h, --help                 Show this help message

EXAMPLES:
    $0                         # Run with defaults
    $0 --skip-build            # Skip image build step
    $0 --no-cleanup -v         # Keep cluster after test with verbose output

EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -c|--cluster-name)
            CLUSTER_NAME="$2"
            shift 2
            ;;
        -t|--tag)
            IMAGE_TAG="$2"
            shift 2
            ;;
        --skip-build)
            SKIP_BUILD=true
            shift
            ;;
        --no-cleanup)
            CLEANUP_ON_EXIT=false
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Enable verbose output if requested
if [ "$VERBOSE" = true ]; then
    set -x
fi

# Cleanup function
cleanup() {
    if [ "$CLEANUP_ON_EXIT" = true ]; then
        print_step "Cleaning up"
        if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
            print_warning "Deleting Kind cluster: ${CLUSTER_NAME}"
            kind delete cluster --name "${CLUSTER_NAME}"
        fi
        
        # Clean up docker registry if it exists
        if docker ps -a --format "{{.Names}}" | grep -q "^${REGISTRY_NAME}$"; then
            print_warning "Stopping and removing registry container"
            docker stop "${REGISTRY_NAME}" || true
            docker rm "${REGISTRY_NAME}" || true
        fi
    else
        print_warning "Skipping cleanup - cluster ${CLUSTER_NAME} is still running"
        echo "To cleanup manually later, run:"
        echo "  kind delete cluster --name ${CLUSTER_NAME}"
        echo "  docker stop ${REGISTRY_NAME} && docker rm ${REGISTRY_NAME}"
    fi
}

# Set trap for cleanup
trap cleanup EXIT

# Check prerequisites
print_step "Checking prerequisites"

# Check if required tools are installed
for tool in kind docker kubectl go; do
    if ! command -v $tool &> /dev/null; then
        print_error "$tool is not installed or not in PATH"
        exit 1
    fi
done

print_success "All required tools are available"

# Check if we're in the right directory
if [ ! -f "go.mod" ] || [ ! -f "Makefile" ] || [ ! -d "api" ]; then
    print_error "This script must be run from the llama-stack-k8s-operator root directory"
    exit 1
fi

print_success "Running from correct directory"

# Create Kind configuration
print_step "Creating Kind cluster configuration"

cat > kind-config.yaml << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        system-reserved: memory=500Mi
        eviction-hard: memory.available<200Mi
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
EOF

print_success "Kind configuration created"

# Delete existing cluster if it exists
if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
    print_warning "Cluster ${CLUSTER_NAME} already exists, deleting it"
    kind delete cluster --name "${CLUSTER_NAME}"
fi

# Create registry container if it doesn't exist
print_step "Setting up local Docker registry"

if ! docker ps --format "{{.Names}}" | grep -q "^${REGISTRY_NAME}$"; then
    # Stop and remove if it exists but is not running
    if docker ps -a --format "{{.Names}}" | grep -q "^${REGISTRY_NAME}$"; then
        print_warning "Removing existing registry container"
        docker stop "${REGISTRY_NAME}" || true
        docker rm "${REGISTRY_NAME}" || true
    fi
    
    # Check if registry image exists locally, if not try to pull it
    if ! docker image inspect registry:2.8 > /dev/null 2>&1; then
        print_warning "Registry image not found locally, pulling from Docker Hub..."
        if ! docker pull registry:2.8; then
            print_error "Failed to pull registry image from Docker Hub"
            print_error "This might be due to Docker Hub rate limiting or authentication issues"
            print_error "Solutions:"
            print_error "  1. Try: docker login"
            print_error "  2. Wait a few minutes and try again (rate limiting)"
            print_error "  3. Use a different registry image if you have one locally"
            exit 1
        fi
    fi
    
    print_warning "Creating local registry container"
    if ! docker run -d --restart=always -p "127.0.0.1:${REGISTRY_PORT}:5000" --name "${REGISTRY_NAME}" registry:2.8; then
        print_error "Failed to create registry container"
        exit 1
    fi
    
    # Wait for registry to be ready
    print_warning "Waiting for registry to be ready..."
    for i in {1..30}; do
        if curl -s "http://localhost:${REGISTRY_PORT}/v2/" > /dev/null 2>&1; then
            break
        fi
        if [ $i -eq 30 ]; then
            print_error "Registry failed to start within 30 seconds"
            print_error "Check registry logs: docker logs ${REGISTRY_NAME}"
            exit 1
        fi
        sleep 1
    done
fi

print_success "Local registry is ready"

# Create Kind cluster
print_step "Creating Kind cluster: ${CLUSTER_NAME}"

kind create cluster --name "${CLUSTER_NAME}" --config kind-config.yaml

# Connect the registry to the cluster network if not already connected
if [ "$(docker inspect -f='{{json .NetworkSettings.Networks.kind}}' "${REGISTRY_NAME}")" = 'null' ]; then
    docker network connect "kind" "${REGISTRY_NAME}"
fi

# Document the local registry in the cluster
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${REGISTRY_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

print_success "Kind cluster created and registry connected"

# Build and push operator image
if [ "$SKIP_BUILD" = false ]; then
    print_step "Building operator image"
    
    IMAGE_NAME="localhost:${REGISTRY_PORT}/llama-stack-k8s-operator:${IMAGE_TAG}"
    
    docker build -t "${IMAGE_NAME}" -f Dockerfile .
    
    print_step "Pushing operator image to local registry"
    docker push "${IMAGE_NAME}"
    
    print_success "Operator image built and pushed"
else
    print_warning "Skipping image build"
    IMAGE_NAME="localhost:${REGISTRY_PORT}/llama-stack-k8s-operator:${IMAGE_TAG}"
fi

# Deploy the operator
print_step "Deploying operator to cluster"

make deploy IMG="${IMAGE_NAME}"

# Wait for operator deployment to be ready
print_step "Waiting for operator deployment to be ready"

if ! kubectl wait --for=condition=available --timeout=300s deployment/llama-stack-k8s-operator-controller-manager -n llama-stack-k8s-operator-system; then
    print_error "Deployment failed to become ready. Debugging information:"
    kubectl describe deployment llama-stack-k8s-operator-controller-manager -n llama-stack-k8s-operator-system
    kubectl logs -l control-plane=controller-manager -n llama-stack-k8s-operator-system --tail=100
    kubectl get events -n llama-stack-k8s-operator-system --sort-by='.lastTimestamp'
    exit 1
fi

print_success "Operator deployment is ready"

# Deploy Ollama (prerequisite for e2e tests)
print_step "Deploying Ollama for e2e tests"

./hack/deploy-quickstart.sh

print_success "Ollama deployed successfully"

# Run e2e tests
print_step "Running e2e tests"

if ! go test -v ./tests/e2e/ -run ^TestE2E -v; then
    print_error "E2E tests failed"
    
    # Collect diagnostic information
    print_step "Collecting diagnostic information"
    
    mkdir -p logs
    
    kubectl -n llama-stack-test get all -o yaml > logs/all.log 2>/dev/null || true
    kubectl -n llama-stack-k8s-operator-system logs deployment.apps/llama-stack-k8s-operator-controller-manager > logs/controller-manager.log 2>/dev/null || true
    kubectl -n llama-stack-test describe all > logs/all-describe.log 2>/dev/null || true
    kubectl -n llama-stack-test describe events > logs/events.log 2>/dev/null || true
    kubectl get llamastackdistributions --all-namespaces -o yaml > logs/llamastackdistributions.log 2>/dev/null || true
    
    print_warning "Diagnostic logs saved to ./logs/ directory"
    print_warning "Check the logs for more details about the failure"
    
    exit 1
fi

print_success "E2E tests passed!"

# Cleanup is handled by the trap

print_success "Local e2e test run completed successfully!"
echo ""
echo "Summary:"
echo "  ✓ Kind cluster: ${CLUSTER_NAME}"
echo "  ✓ Registry: localhost:${REGISTRY_PORT}"
echo "  ✓ Operator image: ${IMAGE_NAME}"
echo "  ✓ E2E tests: PASSED" 