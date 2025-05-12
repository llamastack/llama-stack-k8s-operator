# llama-stack-operator
This repo hosts a kubernetes operator that is responsible for creating and managing [llama-stack](https://github.com/meta-llama/llama-stack) server.


## Features

- Automated deployment of Llama Stack servers
- Support for multiple [distributions](https://github.com/meta-llama/llama-stack?tab=readme-ov-file#distributions) (includes Ollama, vLLM, and others)
- Customizable server configurations
- Volume management for model storage
- Kubernetes-native resource management

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Developer Guide](#developer-guide)
    - [Building the Operator](#building-the-operator)
    - [Deployment](#deployment)
- [Deploying Llama Stack Server](#deploying-the-llama-stack-server)
- [Running E2E Tests](#running-e2e-tests)


## Prerequisites

- Kubernetes cluster (v1.20 or later)
- Go version **go1.23**
- operator-sdk **v1.39.2** (v4 layout) or newer
- kubectl configured to access your cluster
- A running inference server:
  - For local development, you can use the provided script: `/hack/deploy-ollama.sh`

#### Build Image

- Custom operator image can be built using your local repository

  ```commandline
  make image IMG=quay.io/<username>/llama-stack-k8s-operator:<custom-tag>
  ```

  The default image used is `quay.io/opendatahub/llama-stack-k8s-operator:latest` when not supply argument for `make image`


- Once the image is created, the operator can be deployed either directly, or through OLM. For each deployment method a
  kubeconfig should be exported

  ```commandline
  export KUBECONFIG=<path to kubeconfig>
  ```

#### Deployment

**Deploying operator locally**

- Deploy the created image in your cluster using following command:

  ```commandline
  make deploy IMG=quay.io/<username>/llama-stack-k8s-operator:<custom-tag>
  ```

- To remove resources created during installation use:

  ```commandline
  make undeploy
  ```

### Deploying the Llama Stack Server

1. Deploy Inference provider server (ollama, vllm etc)
2. Create LlamaStackDistribution CR to get the server running. Example-
```

2. Deploy an inference provider (e.g., Ollama)

3. Create a LlamaStackDistribution resource:
```yaml
apiVersion: llama.x-k8s.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: llamastackdistribution-sample
  namespace: <your-namespace>
spec:
  replicas: 1
  server:
    distribution:
      name: ollama
    containerSpec:
      port: 8321
      env:
      - name: INFERENCE_MODEL
        value: "meta-llama/Llama-3.2-3B-Instruct"
      - name: OLLAMA_URL
        value: "http://ollama-server-service.default.svc.cluster.local:11434"
    podOverrides:
      volumes:
      - name: llama-storage
        emptyDir: {}
      volumeMounts:
      - name: llama-storage
        mountPath: "/root/.llama"
```
3. Verify the server pod is running in the user define namespace.

### Running E2E Tests

The operator includes end-to-end (E2E) tests to verify the complete functionality of the operator. To run the E2E tests:

1. Ensure you have a running Kubernetes cluster
2. Run the E2E tests using one of the following commands:
   - If you want to deploy the operator and run tests:
     ```commandline
     make deploy e2e-tests
     ```
   - If the operator is already deployed:
     ```commandline
     make e2e-tests
     ```

The make target will handle prerequisites including deploying ollama server.
