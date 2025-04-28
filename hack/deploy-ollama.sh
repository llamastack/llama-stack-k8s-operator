#!/bin/bash

set -e

OLLAMA_IMAGE=${1:-"ollama/ollama:latest"}

echo "Checking if namespace ollama-dist exists..."
if ! oc get namespace ollama-dist &> /dev/null; then
    echo "Creating namespace ollama-dist..."
    oc create namespace ollama-dist
else
    echo "Namespace ollama-dist already exists"
fi

echo "Creating ServiceAccount and setting SCC..."
oc create sa llama-sa -n ollama-dist || true
oc adm policy add-scc-to-user anyuid -z llama-sa -n ollama-dist

echo "Creating Ollama deployment and service with image: $OLLAMA_IMAGE..."
cat <<EOF | oc apply -f -
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
oc rollout status deployment/ollama-server -n ollama-dist

POD_NAME=$(oc get pods -n ollama-dist -l app=ollama-server -o jsonpath="{.items[0].metadata.name}")

echo "Running llama3.2:1b model..."
oc exec -n ollama-dist $POD_NAME -- ollama run llama3.2:1b --keepalive 60m