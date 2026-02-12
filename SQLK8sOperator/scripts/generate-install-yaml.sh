#!/bin/bash
# =============================================================================
# Generate install.yaml - Combines all deployment manifests into a single file
# =============================================================================
# This script generates a single install.yaml file that can be applied directly
# from a URL for easy operator installation.
#
# Usage:
#   ./scripts/generate-install-yaml.sh [VERSION] [IMAGE]
#
# Examples:
#   ./scripts/generate-install-yaml.sh
#   ./scripts/generate-install-yaml.sh v1.0.0
#   ./scripts/generate-install-yaml.sh v1.0.0 ghcr.io/myorg/mssql-operator:v1.0.0
# =============================================================================

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
OUTPUT_FILE="${ROOT_DIR}/install.yaml"
VERSION="${1:-v1.0.0}"
IMAGE="${2:-ghcr.io/tejasaks/mssql-operator:${VERSION}}"

# Source files in order
DEPLOY_DIR="${ROOT_DIR}/deploy"
CRD_DIR="${DEPLOY_DIR}/crds"

echo "Generating install.yaml..."
echo "  Version: ${VERSION}"
echo "  Image: ${IMAGE}"
echo "  Output: ${OUTPUT_FILE}"
echo ""

# Start with header
cat > "${OUTPUT_FILE}" << EOF
# =============================================================================
# SQL Server Kubernetes Operator - Combined Installation Manifest
# =============================================================================
# Version: ${VERSION}
# Generated: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
#
# This file contains all resources needed to install the SQL Server Kubernetes
# Operator. It is auto-generated from the deploy/ directory.
#
# INSTALLATION:
#   kubectl apply -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml
#
# Or download and apply:
#   wget https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml
#   kubectl apply -f install.yaml
#
# VERIFICATION:
#   kubectl get pods -n mssql-system
#   kubectl get crds | grep mssql
#
# UNINSTALLATION:
#   kubectl delete sqlservers --all -A
#   kubectl delete sqlserverags --all -A
#   kubectl delete -f install.yaml
#
# =============================================================================
EOF

# Function to add a file with separator
add_manifest() {
    local file="$1"
    local description="$2"
    
    if [[ -f "$file" ]]; then
        echo "" >> "${OUTPUT_FILE}"
        echo "# -----------------------------------------------------------------------------" >> "${OUTPUT_FILE}"
        echo "# ${description}" >> "${OUTPUT_FILE}"
        echo "# Source: ${file#${ROOT_DIR}/}" >> "${OUTPUT_FILE}"
        echo "# -----------------------------------------------------------------------------" >> "${OUTPUT_FILE}"
        cat "$file" >> "${OUTPUT_FILE}"
        echo "  Added: ${file#${ROOT_DIR}/}"
    else
        echo "  WARNING: File not found: ${file}"
    fi
}

# Add namespaces first
add_manifest "${DEPLOY_DIR}/namespace.yaml" "Namespaces"

# Add service account
add_manifest "${DEPLOY_DIR}/serviceaccount.yaml" "Service Account"

# Add RBAC
add_manifest "${DEPLOY_DIR}/rbac.yaml" "RBAC (ClusterRole and ClusterRoleBinding)"

# Add CRDs
echo ""
echo "Adding CRDs..."
for crd_file in "${CRD_DIR}"/*.yaml; do
    if [[ -f "$crd_file" ]]; then
        crd_name=$(basename "$crd_file")
        add_manifest "$crd_file" "CRD: ${crd_name}"
    fi
done

# Add webhook if exists
if [[ -f "${DEPLOY_DIR}/webhook.yaml" ]]; then
    add_manifest "${DEPLOY_DIR}/webhook.yaml" "Webhook Configuration"
fi

# Add deployment last (with image substitution)
echo ""
echo "Adding deployment with image substitution..."
echo "" >> "${OUTPUT_FILE}"
echo "# -----------------------------------------------------------------------------" >> "${OUTPUT_FILE}"
echo "# Operator Deployment" >> "${OUTPUT_FILE}"
echo "# Source: deploy/deployment.yaml" >> "${OUTPUT_FILE}"
echo "# -----------------------------------------------------------------------------" >> "${OUTPUT_FILE}"

# Read deployment and substitute image
if [[ -f "${DEPLOY_DIR}/deployment.yaml" ]]; then
    # Use sed to replace the image placeholder with actual image,
    # and override imagePullPolicy from Never (local dev) to IfNotPresent (remote install)
    sed -e "s|image:.*mssql-operator.*|image: ${IMAGE}|g" \
        -e "s|imagePullPolicy: Never|imagePullPolicy: IfNotPresent|g" \
        "${DEPLOY_DIR}/deployment.yaml" >> "${OUTPUT_FILE}"
    echo "  Added: deploy/deployment.yaml (with image: ${IMAGE})"
else
    echo "  ERROR: deployment.yaml not found!"
    exit 1
fi

# Add footer
echo "" >> "${OUTPUT_FILE}"
echo "# ==============================================================================" >> "${OUTPUT_FILE}"
echo "# End of install.yaml" >> "${OUTPUT_FILE}"
echo "# ==============================================================================" >> "${OUTPUT_FILE}"

# Calculate stats
LINE_COUNT=$(wc -l < "${OUTPUT_FILE}")
FILE_SIZE=$(du -h "${OUTPUT_FILE}" | cut -f1)

echo ""
echo "=========================================="
echo "install.yaml generated successfully!"
echo "  Lines: ${LINE_COUNT}"
echo "  Size: ${FILE_SIZE}"
echo "  Path: ${OUTPUT_FILE}"
echo ""
echo "To install the operator:"
echo "  kubectl apply -f install.yaml"
echo ""
echo "Or directly from GitHub:"
echo "  kubectl apply -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml"
echo "=========================================="
