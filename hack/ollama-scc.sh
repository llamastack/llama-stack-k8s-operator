#!/usr/bin/env bash

set -e

echo "Checking if OpenShift prerequisites exist..."
if ! kubectl get rolebinding ollama-rolebinding -n ollama-dist &> /dev/null; then
    echo "Creating ollama-role and ollama-rolebinding for ollama-sa"
    cat <<EOF | kubectl apply -f -
apiVersion: security.openshift.io/v1
kind: SecurityContextConstraints
metadata:
  name: ollama-scc
allowPrivilegeEscalation: true
allowPrivilegedContainer: false
allowHostNetwork: false
allowedCapabilities:
- NET_BIND_SERVICE
defaultAddCapabilities: null
fsGroup:
  type: RunAsAny
groups: []
readOnlyRootFilesystem: false
requiredDropCapabilities:
- ALL
runAsUser:
  type: MustRunAs
  uid: 0
seLinuxContext:
  type: MustRunAs
supplementalGroups:
  type: RunAsAny
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  creationTimestamp: null
  name: ollama-role
  namespace: ollama-dist
rules:
- apiGroups:
  - security.openshift.io
  resourceNames:
  - ollama-scc
  resources:
  - securitycontextconstraints
  verbs:
  - use
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ollama-rolebinding
  namespace:  ollama-dist
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: ollama-role
subjects:
- kind: ServiceAccount
  name: ollama-sa
  namespace: ollama-dist
EOF
fi

echo "Annotating ServiceAccount to clarify that it uses ollama-scc..."
kubectl annotate sa ollama-sa -n ollama-dist openshift.io/scc=ollama-scc --overwrite
