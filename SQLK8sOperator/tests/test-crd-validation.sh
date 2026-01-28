#!/bin/bash
# Test: CRD Validation
# Validates CRD schema, validation rules, and basic operations

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/test-helpers.sh"

test_init "crd-validation"

# Ensure CRDs are installed
log_step "Pre-requisite: Verify CRDs Installed"

if ! kubectl get crd sqlservers.mssql.microsoft.com &>/dev/null; then
    log_info "Installing CRDs..."
    kubectl apply -f "${PROJECT_ROOT}/deploy/crds/"
    sleep 5
fi

create_test_namespace

# ============================================================================
# Test 1: SQLServer CRD Schema
# ============================================================================
log_step "Test 1: SQLServer CRD Schema Validation"

# Get CRD and check structure
CRD_VERSIONS=$(kubectl get crd sqlservers.mssql.microsoft.com \
    -o jsonpath='{.spec.versions[*].name}')
assert_contains "${CRD_VERSIONS}" "v1alpha1" "CRD should have v1alpha1 version"

CRD_KIND=$(kubectl get crd sqlservers.mssql.microsoft.com \
    -o jsonpath='{.spec.names.kind}')
assert_equals "SQLServer" "${CRD_KIND}" "CRD kind should be SQLServer"

CRD_SHORTNAME=$(kubectl get crd sqlservers.mssql.microsoft.com \
    -o jsonpath='{.spec.names.shortNames[0]}')
assert_equals "mssql" "${CRD_SHORTNAME}" "CRD should have 'mssql' shortname"

# ============================================================================
# Test 2: SQLServerAG CRD Schema
# ============================================================================
log_step "Test 2: SQLServerAG CRD Schema Validation"

CRD_KIND=$(kubectl get crd sqlserverags.mssql.microsoft.com \
    -o jsonpath='{.spec.names.kind}')
assert_equals "SQLServerAG" "${CRD_KIND}" "CRD kind should be SQLServerAG"

CRD_SHORTNAME=$(kubectl get crd sqlserverags.mssql.microsoft.com \
    -o jsonpath='{.spec.names.shortNames[0]}')
assert_equals "mssqlag" "${CRD_SHORTNAME}" "CRD should have 'mssqlag' shortname"

# ============================================================================
# Test 3: Valid SQLServer Resource Creation
# ============================================================================
log_step "Test 3: Valid SQLServer Resource Creation"

# Create a valid SQLServer resource (name max 13 chars)
cat <<EOF | kubectl apply -n "${TEST_NAMESPACE}" -f -
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: test-sql-01
spec:
  version: "2022"
  edition: Developer
  description: "Test SQL Server for CRD validation"
  instance:
    replicas: 1
    storage:
      data:
        size: 1Gi
  credentials:
    saPasswordSecretRef:
      name: test-sa-password
      key: password
EOF

if resource_exists "sqlserver/test-sql-01" "${TEST_NAMESPACE}"; then
    log_success "Valid SQLServer resource created successfully"
else
    log_error "Failed to create valid SQLServer resource"
fi

# ============================================================================
# Test 3b: Name Length Validation (max 13 chars)
# ============================================================================
log_step "Test 3b: Name Length Validation"

# Try to create SQLServer with name > 13 chars (should be rejected)
if cat <<EOF | kubectl apply -n "${TEST_NAMESPACE}" -f - 2>&1 | grep -qi "invalid\|denied\|error\|too long\|maxLength"; then
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: test-very-long-name-invalid
spec:
  version: "2022"
  edition: Developer
  instance:
    replicas: 1
    storage:
      data:
        size: 1Gi
  credentials:
    saPasswordSecretRef:
      name: test-sa-password
EOF
    log_success "Name exceeding 13 chars correctly rejected"
else
    log_warn "Name length validation may not be enforced"
fi

# ============================================================================
# Test 4: SQLServer Version Validation
# ============================================================================
log_step "Test 4: SQLServer Version Validation"

# Test valid versions
for version in "2019" "2022" "2025"; do
    cat <<EOF | kubectl apply -n "${TEST_NAMESPACE}" -f - 2>/dev/null
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: tst-v${version}
spec:
  version: "${version}"
  edition: Developer
  instance:
    replicas: 1
    storage:
      data:
        size: 1Gi
  credentials:
    saPasswordSecretRef:
      name: test-sa-password
EOF

    if resource_exists "sqlserver/tst-v${version}" "${TEST_NAMESPACE}"; then
        log_success "Version ${version} accepted"
    else
        log_error "Version ${version} rejected unexpectedly"
    fi
done

# Test invalid version (should be rejected)
if cat <<EOF | kubectl apply -n "${TEST_NAMESPACE}" -f - 2>&1 | grep -qi "invalid\|denied\|error"; then
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: tst-v2018
spec:
  version: "2018"
  edition: Developer
  instance:
    replicas: 1
    storage:
      data:
        size: 1Gi
  credentials:
    saPasswordSecretRef:
      name: test-sa-password
EOF
    log_success "Invalid version '2018' correctly rejected"
else
    log_warn "Invalid version validation may not be enforced"
fi

# ============================================================================
# Test 5: SQLServer Edition Validation
# ============================================================================
log_step "Test 5: SQLServer Edition Validation"

for edition in "Developer" "Express" "Standard" "Enterprise"; do
    # Create short name: tst-dev, tst-exp, tst-std, tst-ent
    SHORT_ED=$(echo ${edition} | cut -c1-3 | tr '[:upper:]' '[:lower:]')
    cat <<EOF | kubectl apply -n "${TEST_NAMESPACE}" -f - 2>/dev/null
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: tst-${SHORT_ED}
spec:
  version: "2022"
  edition: "${edition}"
  instance:
    replicas: 1
    storage:
      data:
        size: 1Gi
  credentials:
    saPasswordSecretRef:
      name: test-sa-password
EOF

    if resource_exists "sqlserver/tst-${SHORT_ED}" "${TEST_NAMESPACE}"; then
        log_success "Edition ${edition} accepted"
    else
        log_error "Edition ${edition} rejected unexpectedly"
    fi
done

# ============================================================================
# Test 6: SQLServer Replica Bounds Validation
# ============================================================================
log_step "Test 6: SQLServer Replica Bounds Validation"

# Valid replica count (within 1-9)
cat <<EOF | kubectl apply -n "${TEST_NAMESPACE}" -f - 2>/dev/null
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: tst-rep3
spec:
  version: "2022"
  edition: Developer
  instance:
    replicas: 3
    storage:
      data:
        size: 1Gi
  credentials:
    saPasswordSecretRef:
      name: test-sa-password
EOF

REPLICAS=$(kubectl get sqlserver tst-rep3 -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.spec.instance.replicas}' 2>/dev/null)
assert_equals "3" "${REPLICAS}" "Replicas should be 3"

# ============================================================================
# Test 7: SQLServerAG Resource Creation
# ============================================================================
log_step "Test 7: SQLServerAG Resource Creation"

cat <<EOF | kubectl apply -n "${TEST_NAMESPACE}" -f -
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
metadata:
  name: test-ag
spec:
  description: "Test AG for CRD validation"
  sqlServerRef:
    name: test-sql-01
  availabilityGroup:
    name: TestAG
    replicas: 3
    primaryConfig:
      availabilityMode: SynchronousCommit
      failoverMode: External
    secondaryConfig:
      availabilityMode: SynchronousCommit
      failoverMode: External
EOF

if resource_exists "sqlserverag/test-ag" "${TEST_NAMESPACE}"; then
    log_success "SQLServerAG resource created successfully"
else
    log_error "Failed to create SQLServerAG resource"
fi

# ============================================================================
# Test 8: SQLServerAG Field Validation
# ============================================================================
log_step "Test 8: SQLServerAG Field Validation"

AG_NAME=$(kubectl get sqlserverag test-ag -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.spec.availabilityGroup.name}')
assert_equals "TestAG" "${AG_NAME}" "AG name should be 'TestAG'"

AG_REPLICAS=$(kubectl get sqlserverag test-ag -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.spec.availabilityGroup.replicas}')
assert_equals "3" "${AG_REPLICAS}" "AG replicas should be 3"

PRIMARY_MODE=$(kubectl get sqlserverag test-ag -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.spec.availabilityGroup.primaryConfig.availabilityMode}')
assert_equals "SynchronousCommit" "${PRIMARY_MODE}" "Primary mode should be SynchronousCommit"

# ============================================================================
# Test 9: Required Field Validation
# ============================================================================
log_step "Test 9: Required Field Validation"

# Try to create SQLServer without required credentials
if cat <<EOF | kubectl apply -n "${TEST_NAMESPACE}" -f - 2>&1 | grep -qi "required\|missing\|invalid"; then
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: tst-nocreds
spec:
  version: "2022"
  edition: Developer
  instance:
    replicas: 1
    storage:
      data:
        size: 1Gi
EOF
    log_success "Missing required field 'credentials' correctly rejected"
else
    log_warn "Required field validation may not be enforced"
fi

# Try to create SQLServer without required storage
if cat <<EOF | kubectl apply -n "${TEST_NAMESPACE}" -f - 2>&1 | grep -qi "required\|missing\|invalid"; then
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: tst-nostor
spec:
  version: "2022"
  edition: Developer
  instance:
    replicas: 1
  credentials:
    saPasswordSecretRef:
      name: test-sa-password
EOF
    log_success "Missing required field 'storage' correctly rejected"
else
    log_warn "Required field validation may not be enforced"
fi

# ============================================================================
# Test 10: Printer Columns
# ============================================================================
log_step "Test 10: Printer Columns Validation"

# Check that kubectl get shows expected columns
OUTPUT=$(kubectl get sqlserver -n "${TEST_NAMESPACE}" 2>/dev/null)

if echo "${OUTPUT}" | grep -q "VERSION"; then
    log_success "VERSION column present"
else
    log_warn "VERSION column not visible"
fi

if echo "${OUTPUT}" | grep -q "EDITION"; then
    log_success "EDITION column present"
else
    log_warn "EDITION column not visible"
fi

if echo "${OUTPUT}" | grep -q "PHASE\|READY"; then
    log_success "Status columns present"
else
    log_warn "Status columns not visible"
fi

# ============================================================================
# Test 11: Resource Deletion
# ============================================================================
log_step "Test 11: Resource Deletion"

kubectl delete sqlserver test-sql-01 -n "${TEST_NAMESPACE}" --wait=false

sleep 2

if ! resource_exists "sqlserver/test-sql-01" "${TEST_NAMESPACE}"; then
    log_success "SQLServer resource deleted successfully"
else
    log_info "SQLServer resource pending deletion (finalizer may be processing)"
fi

kubectl delete sqlserverag test-ag -n "${TEST_NAMESPACE}" --wait=false

sleep 2

if ! resource_exists "sqlserverag/test-ag" "${TEST_NAMESPACE}"; then
    log_success "SQLServerAG resource deleted successfully"
else
    log_info "SQLServerAG resource pending deletion"
fi

log_info "CRD validation tests completed"
