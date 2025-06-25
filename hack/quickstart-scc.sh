#!/usr/bin/env bash

set -e

# parameters from caller deploy-quickstart.sh
PROVIDER="${1:-ollama}"
NAMESPACE="${PROVIDER}-dist"
SERVICE_ACCOUNT="${PROVIDER}-sa"
SCC_NAME="${PROVIDER}-scc"

echo "Checking if OpenShift prerequisites exist for namespace: ${NAMESPACE}, service account: ${SERVICE_ACCOUNT}..."
if ! kubectl get scc ${SCC_NAME} &> /dev/null || \
   ! kubectl get sa ${SERVICE_ACCOUNT} -n ${NAMESPACE} &> /dev/null || \
   ! kubectl get role ${SCC_NAME}-role -n ${NAMESPACE} &> /dev/null || \
   ! kubectl get rolebinding ${SCC_NAME}-rolebinding -n ${NAMESPACE} &> /dev/null; then
    echo "Creating ${SCC_NAME}-role and ${SCC_NAME}-rolebinding for ${SERVICE_ACCOUNT}"
    cat <<EOF | kubectl apply -f -
apiVersion: security.openshift.io/v1
kind: SecurityContextConstraints
metadata:
  name: ${SCC_NAME}
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
  name: ${SCC_NAME}-role
  namespace: ${NAMESPACE}
rules:
- apiGroups:
  - security.openshift.io
  resourceNames:
  - ${SCC_NAME}
  resources:
  - securitycontextconstraints
  verbs:
  - use
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ${SCC_NAME}-rolebinding
  namespace: ${NAMESPACE}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: ${SCC_NAME}-role
subjects:
- kind: ServiceAccount
  name: ${SERVICE_ACCOUNT}
  namespace: ${NAMESPACE}
EOF
fi

echo "Annotating ServiceAccount to clarify that it uses ${SCC_NAME}..."
kubectl annotate sa ${SERVICE_ACCOUNT} -n ${NAMESPACE} openshift.io/scc=${SCC_NAME} --overwrite
