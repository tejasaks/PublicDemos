#!/bin/bash
# Test: SQL Server Availability Group Deployment
# Validates end-to-end AG deployment with multiple replicas

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/test-helpers.sh"

test_init "sqlserver-ag"

# Configuration
SA_PASSWORD="TestP@ssw0rd123!"
SQLSERVER_NAME="test-ag-cluster"
AG_NAME="test-ag"
REPLICA_COUNT=3

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

create_secret "mssql-ag-sa-password" "password" "${SA_PASSWORD}" "${TEST_NAMESPACE}"

if resource_exists "secret/mssql-ag-sa-password" "${TEST_NAMESPACE}"; then
    log_success "SA password secret created"
else
    log_error "Failed to create SA password secret"
fi

# ============================================================================
# Test 2: Deploy Multi-Replica SQL Server
# ============================================================================
log_step "Test 2: Deploy Multi-Replica SQL Server (${REPLICA_COUNT} replicas)"

cat <<EOF | kubectl apply -n "${TEST_NAMESPACE}" -f -
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: ${SQLSERVER_NAME}
spec:
  version: "2022"
  edition: Developer
  instance:
    replicas: ${REPLICA_COUNT}
    resources:
      limits:
        cpu: "1"
        memory: 2Gi
      requests:
        cpu: "250m"
        memory: 1Gi
    storage:
      data:
        size: 2Gi
    config:
      agentEnabled: true
      hadrEnabled: true
  credentials:
    saPasswordSecretRef:
      name: mssql-ag-sa-password
      key: password
  service:
    type: ClusterIP
    port: 1433
  monitoring:
    enabled: false
EOF

if resource_exists "sqlserver/${SQLSERVER_NAME}" "${TEST_NAMESPACE}"; then
    log_success "SQLServer resource created with ${REPLICA_COUNT} replicas"
else
    log_error "Failed to create SQLServer resource"
fi

# ============================================================================
# Test 3: Verify StatefulSet Configuration
# ============================================================================
log_step "Test 3: Verify StatefulSet Configuration"

# Wait for StatefulSet to be created
sleep 15

if resource_exists "statefulset/${SQLSERVER_NAME}" "${TEST_NAMESPACE}"; then
    log_success "StatefulSet created"
else
    log_error "StatefulSet not created"
    dump_debug_info
fi

# Check StatefulSet replicas
STS_REPLICAS=$(kubectl get statefulset "${SQLSERVER_NAME}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.spec.replicas}')
assert_equals "${REPLICA_COUNT}" "${STS_REPLICAS}" "StatefulSet should have ${REPLICA_COUNT} replicas"

# Check update strategy
UPDATE_STRATEGY=$(kubectl get statefulset "${SQLSERVER_NAME}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.spec.updateStrategy.type}')
assert_equals "OnDelete" "${UPDATE_STRATEGY}" "Update strategy should be OnDelete"

# Check pod management policy
POD_MGMT_POLICY=$(kubectl get statefulset "${SQLSERVER_NAME}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.spec.podManagementPolicy}')
assert_equals "OrderedReady" "${POD_MGMT_POLICY}" "Pod management policy should be OrderedReady"

# ============================================================================
# Test 4: Wait for All Pods Ready
# ============================================================================
log_step "Test 4: Wait for All Pods Ready"

# This may take a while for 3 replicas
wait_for_pod_ready "app=mssql,mssql.microsoft.com/instance=${SQLSERVER_NAME}" "${TEST_NAMESPACE}" 600 ${REPLICA_COUNT}

# Verify each pod
for i in $(seq 0 $((REPLICA_COUNT - 1))); do
    POD_NAME="${SQLSERVER_NAME}-${i}"
    if kubectl get pod "${POD_NAME}" -n "${TEST_NAMESPACE}" &>/dev/null; then
        POD_STATUS=$(kubectl get pod "${POD_NAME}" -n "${TEST_NAMESPACE}" \
            -o jsonpath='{.status.phase}')
        if [[ "${POD_STATUS}" == "Running" ]]; then
            log_success "Pod ${POD_NAME} is Running"
        else
            log_error "Pod ${POD_NAME} is ${POD_STATUS}"
        fi
    else
        log_error "Pod ${POD_NAME} not found"
    fi
done

# ============================================================================
# Test 5: Verify PVCs for All Replicas
# ============================================================================
log_step "Test 5: Verify PVCs for All Replicas"

for i in $(seq 0 $((REPLICA_COUNT - 1))); do
    PVC_NAME="data-${SQLSERVER_NAME}-${i}"
    if resource_exists "pvc/${PVC_NAME}" "${TEST_NAMESPACE}"; then
        PVC_STATUS=$(kubectl get pvc "${PVC_NAME}" -n "${TEST_NAMESPACE}" \
            -o jsonpath='{.status.phase}')
        if [[ "${PVC_STATUS}" == "Bound" ]]; then
            log_success "PVC ${PVC_NAME} is Bound"
        else
            log_warn "PVC ${PVC_NAME} is ${PVC_STATUS}"
        fi
    else
        log_error "PVC ${PVC_NAME} not found"
    fi
done

# ============================================================================
# Test 6: Deploy Availability Group Configuration
# ============================================================================
log_step "Test 6: Deploy Availability Group Configuration"

cat <<EOF | kubectl apply -n "${TEST_NAMESPACE}" -f -
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
metadata:
  name: ${AG_NAME}
spec:
  sqlServerRef:
    name: ${SQLSERVER_NAME}
  availabilityGroup:
    name: TestAvailabilityGroup
    replicas: ${REPLICA_COUNT}
    primaryConfig:
      availabilityMode: SynchronousCommit
      failoverMode: External
      readableSecondary: ReadOnly
    secondaryConfig:
      availabilityMode: SynchronousCommit
      failoverMode: External
      readableSecondary: ReadOnly
    seedingMode: Automatic
    dbFailover: true
    clusterType: External
    endpointPort: 5022
  failover:
    automatic: true
    dataLossThreshold: 0
    healthCheckTimeout: "30s"
  endpoints:
    primary:
      type: ClusterIP
      port: 1433
    secondary:
      type: ClusterIP
      port: 1433
  sidecar:
    monitorInterval: "10s"
EOF

if resource_exists "sqlserverag/${AG_NAME}" "${TEST_NAMESPACE}"; then
    log_success "SQLServerAG resource created"
else
    log_error "Failed to create SQLServerAG resource"
fi

# ============================================================================
# Test 7: Verify AG Configuration
# ============================================================================
log_step "Test 7: Verify AG Configuration"

# Wait for AG controller to process
sleep 20

AG_AGNAME=$(kubectl get sqlserverag "${AG_NAME}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.spec.availabilityGroup.name}')
assert_equals "TestAvailabilityGroup" "${AG_AGNAME}" "AG name should match"

AG_REPLICAS=$(kubectl get sqlserverag "${AG_NAME}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.spec.availabilityGroup.replicas}')
assert_equals "${REPLICA_COUNT}" "${AG_REPLICAS}" "AG replicas should be ${REPLICA_COUNT}"

FAILOVER_MODE=$(kubectl get sqlserverag "${AG_NAME}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.spec.availabilityGroup.primaryConfig.failoverMode}')
assert_equals "External" "${FAILOVER_MODE}" "Failover mode should be External"

# ============================================================================
# Test 8: Verify AG Endpoint Services
# ============================================================================
log_step "Test 8: Verify AG Endpoint Services"

# Check for primary endpoint service
PRIMARY_SVC="testag-primary"
if resource_exists "service/${PRIMARY_SVC}" "${TEST_NAMESPACE}" 2>/dev/null || \
   resource_exists "service/testavailabilitygroup-primary" "${TEST_NAMESPACE}" 2>/dev/null; then
    log_success "Primary endpoint service created"
else
    log_warn "Primary endpoint service not found (may use different naming)"
fi

# Check for secondary endpoint service
SECONDARY_SVC="testag-secondary"
if resource_exists "service/${SECONDARY_SVC}" "${TEST_NAMESPACE}" 2>/dev/null || \
   resource_exists "service/testavailabilitygroup-secondary" "${TEST_NAMESPACE}" 2>/dev/null; then
    log_success "Secondary endpoint service created"
else
    log_warn "Secondary endpoint service not found"
fi

# ============================================================================
# Test 9: Verify SQLServerAG Status
# ============================================================================
log_step "Test 9: Verify SQLServerAG Status"

# Wait for status update
sleep 30

AG_PHASE=$(kubectl get sqlserverag "${AG_NAME}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.status.phase}')
log_info "AG Phase: ${AG_PHASE}"

if [[ "${AG_PHASE}" == "Synchronized" || "${AG_PHASE}" == "Creating" || "${AG_PHASE}" == "Degraded" ]]; then
    log_success "AG phase is valid: ${AG_PHASE}"
else
    log_warn "AG phase is: ${AG_PHASE}"
fi

# Check for replicas in status
AG_STATUS_REPLICAS=$(kubectl get sqlserverag "${AG_NAME}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.status.replicas}' | jq 'length' 2>/dev/null || echo "0")

if [[ "${AG_STATUS_REPLICAS}" -ge 1 ]]; then
    log_success "AG status shows ${AG_STATUS_REPLICAS} replica(s)"
else
    log_warn "No replicas reported in AG status"
fi

# ============================================================================
# Test 10: Verify HADR Configuration in SQL Server
# ============================================================================
log_step "Test 10: Verify HADR Configuration"

# Check mssql.conf for HADR enabled
CONFIG_DATA=$(kubectl get configmap "${SQLSERVER_NAME}-config" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.data.mssql\.conf}')

if echo "${CONFIG_DATA}" | grep -q "hadrenabled = 1"; then
    log_success "HADR is enabled in mssql.conf"
else
    log_warn "HADR setting not found in mssql.conf"
fi

# ============================================================================
# Test 11: Pod Distribution Check
# ============================================================================
log_step "Test 11: Pod Distribution Check"

# Get nodes running SQL Server pods
NODES=$(kubectl get pods -n "${TEST_NAMESPACE}" \
    -l "app=mssql,mssql.microsoft.com/instance=${SQLSERVER_NAME}" \
    -o jsonpath='{.items[*].spec.nodeName}' | tr ' ' '\n' | sort | uniq)

NODE_COUNT=$(echo "${NODES}" | wc -l)
log_info "Pods distributed across ${NODE_COUNT} node(s)"

if [[ ${NODE_COUNT} -gt 1 ]]; then
    log_success "Pods are distributed across multiple nodes"
else
    log_info "All pods on single node (expected in single-node cluster like minikube)"
fi

# ============================================================================
# Test 12: Scale Down AG
# ============================================================================
log_step "Test 12: Scale Down AG"

NEW_REPLICA_COUNT=$((REPLICA_COUNT - 1))

kubectl patch sqlserver "${SQLSERVER_NAME}" -n "${TEST_NAMESPACE}" \
    --type merge -p "{\"spec\":{\"instance\":{\"replicas\":${NEW_REPLICA_COUNT}}}}"

log_info "Scaled down to ${NEW_REPLICA_COUNT} replicas"

# Wait for scale down
sleep 30

CURRENT_REPLICAS=$(kubectl get statefulset "${SQLSERVER_NAME}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.spec.replicas}')
assert_equals "${NEW_REPLICA_COUNT}" "${CURRENT_REPLICAS}" "StatefulSet should have ${NEW_REPLICA_COUNT} replicas after scale down"

# Also update AG replicas
kubectl patch sqlserverag "${AG_NAME}" -n "${TEST_NAMESPACE}" \
    --type merge -p "{\"spec\":{\"availabilityGroup\":{\"replicas\":${NEW_REPLICA_COUNT}}}}" 2>/dev/null || true

# ============================================================================
# Test 13: Scale Up AG
# ============================================================================
log_step "Test 13: Scale Up AG"

kubectl patch sqlserver "${SQLSERVER_NAME}" -n "${TEST_NAMESPACE}" \
    --type merge -p "{\"spec\":{\"instance\":{\"replicas\":${REPLICA_COUNT}}}}"

log_info "Scaled up to ${REPLICA_COUNT} replicas"

# Wait for scale up
sleep 60

# Wait for new pods
wait_for_pod_ready "app=mssql,mssql.microsoft.com/instance=${SQLSERVER_NAME}" "${TEST_NAMESPACE}" 300 ${REPLICA_COUNT}

CURRENT_REPLICAS=$(kubectl get statefulset "${SQLSERVER_NAME}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.status.readyReplicas}')

if [[ "${CURRENT_REPLICAS}" == "${REPLICA_COUNT}" ]]; then
    log_success "Successfully scaled back up to ${REPLICA_COUNT} replicas"
else
    log_warn "Scale up may still be in progress: ${CURRENT_REPLICAS}/${REPLICA_COUNT} ready"
fi

# ============================================================================
# Test 14: Delete AG First
# ============================================================================
log_step "Test 14: Delete AG Configuration"

kubectl delete sqlserverag "${AG_NAME}" -n "${TEST_NAMESPACE}" --wait=true --timeout=60s

if ! resource_exists "sqlserverag/${AG_NAME}" "${TEST_NAMESPACE}"; then
    log_success "SQLServerAG deleted"
else
    log_warn "SQLServerAG still exists"
fi

# ============================================================================
# Test 15: Delete SQLServer and Verify Cleanup
# ============================================================================
log_step "Test 15: Delete SQLServer and Verify Cleanup"

kubectl delete sqlserver "${SQLSERVER_NAME}" -n "${TEST_NAMESPACE}" --wait=true --timeout=180s

# Wait for StatefulSet to be deleted
sleep 15

if ! resource_exists "statefulset/${SQLSERVER_NAME}" "${TEST_NAMESPACE}"; then
    log_success "StatefulSet deleted"
else
    log_warn "StatefulSet still exists"
fi

# Check pods are deleted
REMAINING_PODS=$(kubectl get pods -n "${TEST_NAMESPACE}" \
    -l "app=mssql,mssql.microsoft.com/instance=${SQLSERVER_NAME}" \
    --no-headers 2>/dev/null | wc -l)

if [[ "${REMAINING_PODS}" -eq 0 ]]; then
    log_success "All pods deleted"
else
    log_warn "${REMAINING_PODS} pods still exist"
fi

# Note: PVCs may be retained based on reclaim policy
REMAINING_PVCS=$(kubectl get pvc -n "${TEST_NAMESPACE}" --no-headers 2>/dev/null | wc -l)
log_info "${REMAINING_PVCS} PVCs remaining (cleanup depends on reclaim policy)"

log_info "SQL Server AG tests completed"
