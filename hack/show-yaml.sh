#!/usr/bin/env bash

set -e

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils.sh"

show_deployment_yaml() {
    local provider="$1"
    local model="$2"

    echo "=== ${provider^^} DEPLOYMENT YAML ==="
    echo ""

    # Load provider configuration
    while IFS='=' read -r key value; do
        if [[ -n "$key" && ! "$key" =~ ^# ]]; then
            eval "$key=\"$value\""
        fi
    done < <(get_provider_config "${provider}")

    # Use provided model or default
    if [ -z "${model}" ]; then
        MODEL="${DEFAULT_MODEL}"
    else
        MODEL="${model}"
    fi

    # Set default args
    if [ "${provider}" = "ollama" ]; then
        INIT_ARGS="ollama serve & sleep 15 && ollama pull ${MODEL}"
        DEFAULT_ARGS="ollama serve"
    else
        DEFAULT_ARGS="vllm serve --dtype auto --model ${MODEL}"
        INIT_ARGS="sleep 1"
    fi

    # Get names
    NAMESPACE=$(get_namespace "${provider}")
    SERVER_NAME=$(get_server_name "${provider}")
    VOLUMN=$(get_volume_name "${provider}")

    # Generate security context manually (without kubectl calls)
    if [ "${provider}" = "ollama" ]; then
        SERVICE_ACCOUNT="${provider}-sa"
        SECURITY_CONTEXT_YAML="      securityContext:
        runAsUser: 0
        runAsGroup: 0
        fsGroup: 0"
        CONTAINER_SECURITY_CONTEXT_YAML="securityContext:
            allowPrivilegeEscalation: true
            runAsNonRoot: false"
        OPENSHIFT_ANNOTATION=""
    else
        SERVICE_ACCOUNT=""
        SECURITY_CONTEXT_YAML=""
        CONTAINER_SECURITY_CONTEXT_YAML="securityContext:
            runAsNonRoot: true"
        OPENSHIFT_ANNOTATION="      annotations:
        openshift.io/required-scc: restricted-v2"
    fi

    # Convert env vars
    ENV_YAML=$(convert_env_to_yaml "${DEFAULT_ENV_VARS}")

    # Generate the YAML
    cat <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${SERVER_NAME}
  namespace: ${NAMESPACE}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ${SERVER_NAME}
  template:
    metadata:
      labels:
        app: ${SERVER_NAME}
${OPENSHIFT_ANNOTATION}
    spec:
      initContainers:
        - name: ${SERVER_NAME}-downloadmodel
          command: ${COMMAND}
          imagePullPolicy: Always
          volumeMounts:
            - name: ${VOLUMN}
              mountPath: /root/.${provider}
          terminationMessagePolicy: File
          image: ${IMAGE}
          args: ["${INIT_ARGS}"]
${SERVICE_ACCOUNT:+      serviceAccountName: ${SERVICE_ACCOUNT}}
${SECURITY_CONTEXT_YAML}
      containers:
        - name: ${SERVER_NAME}
          image: ${IMAGE}
          command: ${COMMAND}
          args: ["${DEFAULT_ARGS}"]
          env:
${ENV_YAML}
          ports:
          - containerPort: ${PORT}
          resources:
            requests:
              cpu: "500m"
              memory: "1Gi"
          readinessProbe:
            httpGet:
              path: ${HEALTH_PATH}
              port: ${PORT}
            initialDelaySeconds: 30
            periodSeconds: 30
            timeoutSeconds: 5
            failureThreshold: 3
            successThreshold: 1
          ${CONTAINER_SECURITY_CONTEXT_YAML}
          volumeMounts:
          - name: ${VOLUMN}
            mountPath: /root/.${provider}
      volumes:
        - name: ${VOLUMN}
          emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: ${SERVER_NAME}-service
  namespace: ${NAMESPACE}
spec:
  selector:
    app: ${SERVER_NAME}
  ports:
  - protocol: TCP
    port: ${PORT}
    targetPort: ${PORT}
  type: ClusterIP
EOF
}

echo "Showing deployment YAML for both providers..."
echo ""

# Show Ollama YAML
show_deployment_yaml "ollama" "llama3.2:1b"

echo ""
echo "=========================================="
echo ""

# Show vLLM YAML
show_deployment_yaml "vllm" "meta-llama/Llama-3.2-1B"
