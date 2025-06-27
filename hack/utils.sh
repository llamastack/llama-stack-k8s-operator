#!/usr/bin/env bash

# Utility functions for deployment scripts

# for the runtime container env variables
convert_env_to_yaml() {
    local env_string="$1"
    local yaml_env=""
    IFS=',' read -ra env_pairs <<< "$env_string"
    for pair in "${env_pairs[@]}"; do
        IFS='=' read -r key value <<< "$pair"
        # Strip outer quotes from value
        value=$(echo "$value" | sed 's/^["'\'']*//;s/["'\'']*$//')

        # Check if value is a secret reference (format: secret:secretname:key) this is for hf-token-secret to download models
        if [[ "$value" =~ ^secret:([^:]+):([^:]+)$ ]]; then
            local secret_name="${BASH_REMATCH[1]}"
            local secret_key="${BASH_REMATCH[2]}"
            if [ -n "$yaml_env" ]; then
                yaml_env="${yaml_env}\n"
            fi
            yaml_env="${yaml_env}            - name: ${key}\n              valueFrom:\n                secretKeyRef:\n                  name: ${secret_name}\n                  key: ${secret_key}"
        else
            if [ -n "$yaml_env" ]; then
                yaml_env="${yaml_env}\n"
            fi
            yaml_env="${yaml_env}            - name: ${key}\n              value: \"${value}\""
        fi
    done
    echo -e "$yaml_env"
}

# can extend later once we support other providers
validate_provider() {
    local provider="$1"
    case "${provider}" in
        "ollama"|"vllm")
            return 0
            ;;
        *)
            echo "Error: Unsupported provider '${provider}'"
            echo "Supported providers: ollama, vllm for now"
            return 1
            ;;
    esac
}

# set default values for supported provider
get_provider_config() {
    local provider="$1"
    case "${provider}" in
        "ollama")
            echo "IMAGE=ollama/ollama:latest"
            echo "INFERENCE_SERVER=ollama"
            echo "COMMAND=[\"/bin/sh\", \"-c\"]"
            echo "DEFAULT_MODEL=llama3.2:1b"
            echo "PORT=11434"
            echo "HEALTH_PATH=/api/version"
            echo "DEFAULT_ENV_VARS=OLLAMA_KEEP_ALIVE=60m"
            ;;
        "vllm")
            echo "IMAGE=vllm/vllm-openai:latest"
            echo "INFERENCE_SERVER=vllm"
            echo "COMMAND=[\"/bin/sh\", \"-c\"]"
            echo "DEFAULT_MODEL=meta-llama/Llama-3.2-1B"
            echo "PORT=8000"
            echo "HEALTH_PATH=/health"
            echo "DEFAULT_ENV_VARS=CUDA_VISIBLE_DEVICES='', VLLM_NO_USAGE_STATS=1, VLLM_TARGET_DEVICE=cpu, VLLM_ENFORCE_EAGER=1, HUGGING_FACE_HUB_TOKEN=secret:hf-token-secret:token"
            ;;
    esac
}

# Generate namespace name
get_namespace() {
    local provider="$1"
    echo "${provider}-dist"
}

# Generate deployment name
get_server_name() {
    local provider="$1"
    echo "${provider}-server"
}

# Generate volume mount name
get_volume_name() {
    local provider="$1"
    echo "${provider}-data"
}

# Generate security related YAML and SCC based on provider
generate_security_context() {
    local provider="$1"
    local namespace="$2"
    local service_account="${provider}-sa"

    # OpenShift requires specific permissions in order for the container to run as uid 0
    if [ "${provider}" = "ollama" ]; then
        # Create ServiceAccount for Ollama (needed for SCC)
        echo "Checking if ServiceAccount ${service_account} exists..."
        if ! kubectl get sa ${service_account} -n ${namespace} &> /dev/null; then
            echo "Creating ServiceAccount ${service_account}..."
            kubectl create sa ${service_account} -n ${namespace}
        else
            echo "ServiceAccount ${service_account} already exists"
        fi
        export SERVICE_ACCOUNT="${service_account}"

        # Generate security context YAML for Ollama (need root)
        SECURITY_CONTEXT_YAML="      securityContext:
        runAsUser: 0
        runAsGroup: 0
        fsGroup: 0"
        CONTAINER_SECURITY_CONTEXT_YAML="securityContext:
            allowPrivilegeEscalation: true
            runAsNonRoot: false"
        OPENSHIFT_ANNOTATION=""

        if kubectl api-resources --api-group=security.openshift.io | grep -iq 'SecurityContextConstraints'; then
            "$(dirname "${BASH_SOURCE[0]}")/quickstart-scc.sh" "${provider}"
        fi
    else
        # For vLLM, use restricted-v2 SCC annotation (do not create new SCC resource)
        SECURITY_CONTEXT_YAML=""
        CONTAINER_SECURITY_CONTEXT_YAML="securityContext:
            runAsNonRoot: true"

        # Add annotation for restricted-v2 SCC to deployment
        if kubectl api-resources --api-group=security.openshift.io | grep -iq 'SecurityContextConstraints'; then
            OPENSHIFT_ANNOTATION="      annotations:
        openshift.io/required-scc: restricted-v2"
        fi
    fi

    # Export variables so they can be used by deploy-quickstart.sh
    export SECURITY_CONTEXT_YAML
    export CONTAINER_SECURITY_CONTEXT_YAML
    export OPENSHIFT_ANNOTATION

}
