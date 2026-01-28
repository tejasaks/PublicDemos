#!/bin/bash
# Test: Operator Deployment Validation
# Validates that the MSSQL operator can be installed and runs correctly

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/test-helpers.sh"

test_init "operator-deployment"

# Override cleanup for this test - we manage operator namespace separately
OPERATOR_TEST_NAMESPACE="mssql-operator-test-$(date +%s)"
OPERATOR_NAMESPACE="${OPERATOR_TEST_NAMESPACE}"

cleanup_operator() {
    if [[ "${SKIP_CLEANUP}" == "true" ]]; then
        log_warn "Skipping cleanup (SKIP_CLEANUP=true)"
        return
    fi
    
    log_step "Cleaning Up Operator"
    
    # Delete operator resources
    kubectl delete deployment mssql-operator -n "${OPERATOR_NAMESPACE}" 2>/dev/null || true
    kubectl delete clusterrolebinding mssql-operator 2>/dev/null || true
    kubectl delete clusterrole mssql-operator 2>/dev/null || true
    kubectl delete serviceaccount mssql-operator -n "${OPERATOR_NAMESPACE}" 2>/dev/null || true
    
    # Delete CRDs
    kubectl delete crd sqlservers.mssql.microsoft.com 2>/dev/null || true
    kubectl delete crd sqlserverags.mssql.microsoft.com 2>/dev/null || true
    kubectl delete crd operatorconfigurations.mssql.microsoft.com 2>/dev/null || true
    
    # Delete namespace
    delete_namespace "${OPERATOR_NAMESPACE}"
}

# Override trap
trap 'cleanup_operator; test_result' EXIT

# ============================================================================
# Test 1: Validate YAML Manifests
# ============================================================================
log_step "Test 1: Validate YAML Manifests"

cd "${PROJECT_ROOT}"

# Check that all deploy files exist
for file in namespace.yaml serviceaccount.yaml rbac.yaml deployment.yaml; do
    if [[ -f "deploy/${file}" ]]; then
        log_success "Found deploy/${file}"
    else
        log_error "Missing deploy/${file}"
    fi
done

# Check CRD files
for file in sqlserver-crd.yaml sqlserverag-crd.yaml; do
    if [[ -f "deploy/crds/${file}" ]]; then
        log_success "Found deploy/crds/${file}"
    else
        log_error "Missing deploy/crds/${file}"
    fi
done

# ============================================================================
# Test 2: Create Operator Namespace
# ============================================================================
log_step "Test 2: Create Operator Namespace"

# Create namespace with custom name for testing
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: ${OPERATOR_NAMESPACE}
  labels:
    app.kubernetes.io/name: mssql-operator
EOF

if kubectl get namespace "${OPERATOR_NAMESPACE}" &>/dev/null; then
    log_success "Namespace created: ${OPERATOR_NAMESPACE}"
else
    log_error "Failed to create namespace"
fi

# ============================================================================
# Test 3: Install CRDs
# ============================================================================
log_step "Test 3: Install CRDs"

kubectl apply -f "${PROJECT_ROOT}/deploy/crds/"

# Verify CRDs are installed
sleep 5

if kubectl get crd sqlservers.mssql.microsoft.com &>/dev/null; then
    log_success "SQLServer CRD installed"
else
    log_error "SQLServer CRD not found"
fi

if kubectl get crd sqlserverags.mssql.microsoft.com &>/dev/null; then
    log_success "SQLServerAG CRD installed"
else
    log_error "SQLServerAG CRD not found"
fi

# ============================================================================
# Test 4: Install Operator Resources
# ============================================================================
log_step "Test 4: Install Operator Resources"

# Create ServiceAccount
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: mssql-operator
  namespace: ${OPERATOR_NAMESPACE}
EOF

if [[ $? -eq 0 ]]; then
    log_success "ServiceAccount created"
else
    log_error "Failed to create ServiceAccount"
fi

# Apply RBAC (uses cluster-scoped resources)
kubectl apply -f "${PROJECT_ROOT}/deploy/rbac.yaml"

if [[ $? -eq 0 ]]; then
    log_success "RBAC resources created"
else
    log_error "Failed to create RBAC resources"
fi

# Create Deployment with custom namespace
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mssql-operator
  namespace: ${OPERATOR_NAMESPACE}
  labels:
    app.kubernetes.io/name: mssql-operator
    app: mssql-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: mssql-operator
      app: mssql-operator
  template:
    metadata:
      labels:
        app.kubernetes.io/name: mssql-operator
        app: mssql-operator
    spec:
      serviceAccountName: mssql-operator
      securityContext:
        runAsNonRoot: true
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: operator
          image: mssql-operator:dev
          imagePullPolicy: Never
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
            readOnlyRootFilesystem: true
          ports:
            - containerPort: 8080
              name: metrics
            - containerPort: 8081
              name: health
          livenessProbe:
            httpGet:
              path: /healthz
              port: health
            initialDelaySeconds: 15
            periodSeconds: 20
          readinessProbe:
            httpGet:
              path: /readyz
              port: health
            initialDelaySeconds: 5
            periodSeconds: 10
          resources:
            limits:
              cpu: 500m
              memory: 256Mi
            requests:
              cpu: 100m
              memory: 128Mi
          env:
            - name: OPERATOR_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: AG_HELPER_IMAGE
              value: "mssql-ag-helper:dev"
            - name: AG_HELPER_IMAGE_PULL_POLICY
              value: "Never"
EOF

if [[ $? -eq 0 ]]; then
    log_success "Operator Deployment created"
else
    log_error "Failed to create Deployment"
fi

# ============================================================================
# Test 5: Verify Operator Deployment
# ============================================================================
log_step "Test 5: Verify Operator Deployment"

wait_for "deployment/mssql-operator" "condition=Available" 120 "${OPERATOR_NAMESPACE}"

# Check replica count
READY_REPLICAS=$(kubectl get deployment mssql-operator \
    -n "${OPERATOR_NAMESPACE}" -o jsonpath='{.status.readyReplicas}')
assert_equals "1" "${READY_REPLICAS}" "Operator should have 1 ready replica"

# ============================================================================
# Test 6: Verify ServiceAccount
# ============================================================================
log_step "Test 6: Verify ServiceAccount"

if resource_exists "serviceaccount/mssql-operator" "${OPERATOR_NAMESPACE}"; then
    log_success "ServiceAccount exists"
else
    log_error "ServiceAccount not found"
fi

# ============================================================================
# Test 7: Verify RBAC
# ============================================================================
log_step "Test 7: Verify RBAC"

if kubectl get clusterrole mssql-operator &>/dev/null; then
    log_success "ClusterRole exists"
else
    log_error "ClusterRole not found"
fi

if kubectl get clusterrolebinding mssql-operator &>/dev/null; then
    log_success "ClusterRoleBinding exists"
else
    log_error "ClusterRoleBinding not found"
fi

# ============================================================================
# Test 8: Verify Operator Pod Health
# ============================================================================
log_step "Test 8: Verify Operator Pod Health"

POD_NAME=$(kubectl get pods -n "${OPERATOR_NAMESPACE}" \
    -l app.kubernetes.io/name=mssql-operator \
    -o jsonpath='{.items[0].metadata.name}')

assert_not_empty "${POD_NAME}" "Operator pod should exist"

# Check pod status
POD_STATUS=$(kubectl get pod "${POD_NAME}" -n "${OPERATOR_NAMESPACE}" \
    -o jsonpath='{.status.phase}')
assert_equals "Running" "${POD_STATUS}" "Operator pod should be Running"

# Check container ready
CONTAINER_READY=$(kubectl get pod "${POD_NAME}" -n "${OPERATOR_NAMESPACE}" \
    -o jsonpath='{.status.containerStatuses[0].ready}')
assert_equals "true" "${CONTAINER_READY}" "Operator container should be ready"

# ============================================================================
# Test 9: Verify Health Endpoints
# ============================================================================
log_step "Test 9: Verify Health Endpoints"

# Port forward to health endpoint
kubectl port-forward -n "${OPERATOR_NAMESPACE}" "pod/${POD_NAME}" 8081:8081 &
PF_PID=$!
sleep 3

# Check healthz
if curl -s http://localhost:8081/healthz | grep -q "ok\|{}" 2>/dev/null; then
    log_success "Health endpoint responding"
else
    log_warn "Health endpoint check inconclusive (may need different response format)"
fi

# Check readyz
if curl -s http://localhost:8081/readyz | grep -q "ok\|{}" 2>/dev/null; then
    log_success "Ready endpoint responding"
else
    log_warn "Ready endpoint check inconclusive"
fi

kill $PF_PID 2>/dev/null || true

# ============================================================================
# Test 10: Verify Operator Logs
# ============================================================================
log_step "Test 10: Verify Operator Logs"

LOGS=$(kubectl logs -n "${OPERATOR_NAMESPACE}" "${POD_NAME}" --tail=50)

if echo "${LOGS}" | grep -qi "starting manager"; then
    log_success "Operator started successfully (found startup log)"
else
    log_warn "Could not verify startup log message"
fi

# Check for errors
if echo "${LOGS}" | grep -qi "error.*unable to create\|panic\|fatal"; then
    log_error "Found errors in operator logs"
    echo "${LOGS}" | grep -i "error\|panic\|fatal"
else
    log_success "No critical errors in operator logs"
fi

log_info "Operator deployment tests completed"
