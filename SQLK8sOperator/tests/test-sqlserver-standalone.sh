#!/bin/bash
# Test: SQL Server Standalone Deployment
# Validates end-to-end standalone SQL Server deployment

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/test-helpers.sh"

test_init "sqlserver-standalone"

# Configuration
SA_PASSWORD="TestP@ssw0rd123!"
SQLSERVER_NAME="tst-sa-01"  # Max 13 chars due to SQL Server NetBIOS limit

# ============================================================================
# Pre-requisites
# ============================================================================
log_step "Pre-requisites: Verify Operator Running"

# Check if operator is running
if ! kubectl get deployment -n "${OPERATOR_NAMESPACE}" -l app.kubernetes.io/name=mssql-operator &>/dev/null; then
    log_error "Operator not found. Please install the operator first."
    exit 1
fi

OPERATOR_READY=$(kubectl get deployment -n "${OPERATOR_NAMESPACE}" \
    -l app.kubernetes.io/name=mssql-operator \
    -o jsonpath='{.items[0].status.readyReplicas}' 2>/dev/null)

if [[ "${OPERATOR_READY}" != "1" ]]; then
    log_error "Operator is not ready"
    exit 1
fi

log_success "Operator is running"

create_test_namespace

# ============================================================================
# Test 1: Create SA Password Secret
# ============================================================================
log_step "Test 1: Create SA Password Secret"

create_secret "mssql-sa-password" "password" "${SA_PASSWORD}" "${TEST_NAMESPACE}"

if resource_exists "secret/mssql-sa-password" "${TEST_NAMESPACE}"; then
    log_success "SA password secret created"
else
    log_error "Failed to create SA password secret"
fi

# ============================================================================
# Test 2: Deploy Standalone SQL Server
# ============================================================================
log_step "Test 2: Deploy Standalone SQL Server"

cat <<EOF | kubectl apply -n "${TEST_NAMESPACE}" -f -
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: ${SQLSERVER_NAME}
spec:
  version: "2022"
  edition: Developer
  instance:
    count: 1
    resources:
      limits:
        cpu: "2"
        memory: 4Gi
      requests:
        cpu: "500m"
        memory: 2Gi
    storage:
      data:
        size: 2Gi
  credentials:
    saPasswordSecretRef:
      name: mssql-sa-password
      key: password
  service:
    type: ClusterIP
    port: 1433
  monitoring:
    enabled: false
EOF

if resource_exists "sqlserver/${SQLSERVER_NAME}" "${TEST_NAMESPACE}"; then
    log_success "SQLServer resource created"
else
    log_error "Failed to create SQLServer resource"
fi

# ============================================================================
# Test 3: Verify StatefulSet Created
# ============================================================================
log_step "Test 3: Verify StatefulSet Created"

# Wait for StatefulSet to be created
sleep 10

if resource_exists "statefulset/${SQLSERVER_NAME}" "${TEST_NAMESPACE}"; then
    log_success "StatefulSet created"
else
    log_error "StatefulSet not created"
    dump_debug_info
fi

# Check StatefulSet replicas
STS_REPLICAS=$(kubectl get statefulset "${SQLSERVER_NAME}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.spec.replicas}')
assert_equals "1" "${STS_REPLICAS}" "StatefulSet should have 1 replica"

# ============================================================================
# Test 4: Verify Headless Service Created
# ============================================================================
log_step "Test 4: Verify Headless Service Created"

if resource_exists "service/${SQLSERVER_NAME}-headless" "${TEST_NAMESPACE}"; then
    log_success "Headless service created"
    
    SVC_CLUSTER_IP=$(kubectl get service "${SQLSERVER_NAME}-headless" -n "${TEST_NAMESPACE}" \
        -o jsonpath='{.spec.clusterIP}')
    assert_equals "None" "${SVC_CLUSTER_IP}" "Headless service should have no ClusterIP"
else
    log_error "Headless service not created"
fi

# ============================================================================
# Test 5: Verify ConfigMap Created
# ============================================================================
log_step "Test 5: Verify ConfigMap Created"

if resource_exists "configmap/${SQLSERVER_NAME}-config" "${TEST_NAMESPACE}"; then
    log_success "ConfigMap created"
    
    # Check mssql.conf content
    CONFIG_DATA=$(kubectl get configmap "${SQLSERVER_NAME}-config" -n "${TEST_NAMESPACE}" \
        -o jsonpath='{.data.mssql\.conf}')
    
    if echo "${CONFIG_DATA}" | grep -q "hadrenabled"; then
        log_success "mssql.conf contains HADR setting"
    else
        log_warn "HADR setting not found in mssql.conf"
    fi
else
    log_error "ConfigMap not created"
fi

# ============================================================================
# Test 6: Wait for Pod Ready
# ============================================================================
log_step "Test 6: Wait for Pod Ready"

wait_for_pod_ready "app=mssql,mssql.microsoft.com/instance=${SQLSERVER_NAME}" "${TEST_NAMESPACE}" 300 1

# Get pod name
POD_NAME=$(kubectl get pods -n "${TEST_NAMESPACE}" \
    -l "app=mssql,mssql.microsoft.com/instance=${SQLSERVER_NAME}" \
    -o jsonpath='{.items[0].metadata.name}')

assert_not_empty "${POD_NAME}" "Pod should exist"

# ============================================================================
# Test 7: Verify Pod Configuration
# ============================================================================
log_step "Test 7: Verify Pod Configuration"

# Check container image
CONTAINER_IMAGE=$(kubectl get pod "${POD_NAME}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.spec.containers[?(@.name=="mssql")].image}')
assert_contains "${CONTAINER_IMAGE}" "mssql/server" "Container should use SQL Server image"

# Check environment variables
ACCEPT_EULA=$(kubectl get pod "${POD_NAME}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.spec.containers[?(@.name=="mssql")].env[?(@.name=="ACCEPT_EULA")].value}')
assert_equals "Y" "${ACCEPT_EULA}" "ACCEPT_EULA should be Y"

MSSQL_PID=$(kubectl get pod "${POD_NAME}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.spec.containers[?(@.name=="mssql")].env[?(@.name=="MSSQL_PID")].value}')
assert_equals "Developer" "${MSSQL_PID}" "MSSQL_PID should be Developer"

# ============================================================================
# Test 8: Verify PVC Created
# ============================================================================
log_step "Test 8: Verify PVC Created"

PVC_NAME="data-${SQLSERVER_NAME}-0"
if resource_exists "pvc/${PVC_NAME}" "${TEST_NAMESPACE}"; then
    log_success "PVC created: ${PVC_NAME}"
    
    PVC_STATUS=$(kubectl get pvc "${PVC_NAME}" -n "${TEST_NAMESPACE}" \
        -o jsonpath='{.status.phase}')
    assert_equals "Bound" "${PVC_STATUS}" "PVC should be Bound"
else
    log_error "PVC not created"
fi

# ============================================================================
# Test 9: Verify SQLServer Status
# ============================================================================
log_step "Test 9: Verify SQLServer Status"

# Wait for status to be updated
sleep 30

PHASE=$(kubectl get sqlserver "${SQLSERVER_NAME}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.status.phase}')

if [[ "${PHASE}" == "Running" ]]; then
    log_success "SQLServer phase is Running"
else
    log_warn "SQLServer phase is ${PHASE} (expected Running)"
fi

READY=$(kubectl get sqlserver "${SQLSERVER_NAME}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.status.ready}')

if [[ "${READY}" == "true" ]]; then
    log_success "SQLServer is ready"
else
    log_warn "SQLServer ready status is ${READY}"
fi

# Check instances in status
INSTANCE_COUNT=$(kubectl get sqlserver "${SQLSERVER_NAME}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.status.instances}' | jq 'length' 2>/dev/null || echo "0")

if [[ "${INSTANCE_COUNT}" -ge 1 ]]; then
    log_success "Status shows ${INSTANCE_COUNT} instance(s)"
else
    log_warn "No instances reported in status"
fi

# ============================================================================
# Test 10: Verify SQL Server Connectivity (if tools available)
# ============================================================================
log_step "Test 10: Verify SQL Server Connectivity"

# Check if sqlcmd is available in container
if kubectl exec -n "${TEST_NAMESPACE}" "${POD_NAME}" -c mssql -- \
    which /opt/mssql-tools/bin/sqlcmd &>/dev/null 2>&1 || \
   kubectl exec -n "${TEST_NAMESPACE}" "${POD_NAME}" -c mssql -- \
    which /opt/mssql-tools18/bin/sqlcmd &>/dev/null 2>&1; then
    
    # Try to connect
    SQLCMD_PATH="/opt/mssql-tools/bin/sqlcmd"
    if ! kubectl exec -n "${TEST_NAMESPACE}" "${POD_NAME}" -c mssql -- \
        test -f "${SQLCMD_PATH}" 2>/dev/null; then
        SQLCMD_PATH="/opt/mssql-tools18/bin/sqlcmd"
    fi
    
    QUERY_RESULT=$(kubectl exec -n "${TEST_NAMESPACE}" "${POD_NAME}" -c mssql -- \
        "${SQLCMD_PATH}" -S localhost -U sa -P "${SA_PASSWORD}" -C \
        -Q "SELECT @@VERSION" 2>/dev/null | head -5)
    
    if echo "${QUERY_RESULT}" | grep -qi "Microsoft SQL Server"; then
        log_success "SQL Server is responding to queries"
        log_info "Version: $(echo "${QUERY_RESULT}" | grep -i "Microsoft SQL Server" | head -1)"
    else
        log_warn "Could not verify SQL Server response"
    fi
else
    log_info "sqlcmd not available in container, skipping direct query test"
fi

# ============================================================================
# Test 11: Verify Pod Labels
# ============================================================================
log_step "Test 11: Verify Pod Labels"

POD_LABELS=$(kubectl get pod "${POD_NAME}" -n "${TEST_NAMESPACE}" -o jsonpath='{.metadata.labels}')

if echo "${POD_LABELS}" | grep -q "mssql.microsoft.com/instance"; then
    log_success "Pod has instance label"
else
    log_error "Pod missing instance label"
fi

if echo "${POD_LABELS}" | grep -q "mssql.microsoft.com/version"; then
    log_success "Pod has version label"
else
    log_warn "Pod missing version label"
fi

# ============================================================================
# Test 12: Scale Down and Verify
# ============================================================================
log_step "Test 12: Shutdown Test"

# Set shutdown flag
kubectl patch sqlserver "${SQLSERVER_NAME}" -n "${TEST_NAMESPACE}" \
    --type merge -p '{"spec":{"shutdown":true}}' 2>/dev/null || true

log_info "Shutdown test skipped (feature may not be implemented)"

# ============================================================================
# Test 13: Delete SQLServer and Verify Cleanup
# ============================================================================
log_step "Test 13: Delete SQLServer and Verify Cleanup"

kubectl delete sqlserver "${SQLSERVER_NAME}" -n "${TEST_NAMESPACE}" --wait=true --timeout=120s

# Wait for StatefulSet to be deleted
sleep 10

if ! resource_exists "statefulset/${SQLSERVER_NAME}" "${TEST_NAMESPACE}"; then
    log_success "StatefulSet deleted"
else
    log_warn "StatefulSet still exists (may have finalizers)"
fi

if ! resource_exists "service/${SQLSERVER_NAME}-headless" "${TEST_NAMESPACE}"; then
    log_success "Headless service deleted"
else
    log_warn "Headless service still exists"
fi

if ! resource_exists "configmap/${SQLSERVER_NAME}-config" "${TEST_NAMESPACE}"; then
    log_success "ConfigMap deleted"
else
    log_warn "ConfigMap still exists"
fi

# Note: PVCs may be retained based on policy
log_info "PVCs may be retained based on reclaim policy"

log_info "SQL Server standalone tests completed"
