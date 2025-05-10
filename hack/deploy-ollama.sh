#!/usr/bin/env bash

set -e

OLLAMA_IMAGE=${1:-"ollama/ollama:latest"}

echo "Checking if namespace ollama-dist exists..."
if ! kubectl get namespace ollama-dist &> /dev/null; then
    echo "Creating namespace ollama-dist..."
    kubectl create namespace ollama-dist
else
    echo "Namespace ollama-dist already exists"
fi

echo "Creating ServiceAccount..."
if ! kubectl get sa llama-sa -n ollama-dist &> /dev/null; then
    echo "Creating ServiceAccount llama-sa..."
    kubectl create sa llama-sa -n ollama-dist
else
    echo "ServiceAccount llama-sa already exists"
fi

echo "Creating Ollama deployment and service with image: $OLLAMA_IMAGE..."
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ollama-server
  namespace: ollama-dist
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ollama-server
  template:
    metadata:
      labels:
        app: ollama-server
    spec:
      serviceAccountName: llama-sa
      securityContext:
        runAsUser: 0
        runAsGroup: 0
        fsGroup: 0
      containers:
      - name: ollama-server
        image: ${OLLAMA_IMAGE}
        args: ["serve"]
        ports:
        - containerPort: 11434
        resources:
          requests:
            cpu: "500m"
            memory: "1Gi"
        securityContext:
          allowPrivilegeEscalation: true
          runAsNonRoot: false
---
apiVersion: v1
kind: Service
metadata:
  name: ollama-server-service
  namespace: ollama-dist
spec:
  selector:
    app: ollama-server
  ports:
  - protocol: TCP
    port: 11434
    targetPort: 11434
  type: ClusterIP
EOF

echo "Waiting for Ollama deployment to be ready..."
kubectl rollout status deployment/ollama-server -n ollama-dist

POD_NAME=$(kubectl get pods -n ollama-dist -l app=ollama-server -o name |  head -n1)

echo "Running llama3.2:1b model..."
kubectl exec -n ollama-dist $POD_NAME -- ollama run llama3.2:1b --keepalive 60m
