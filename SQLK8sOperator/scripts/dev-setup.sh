#!/bin/bash
# Development setup script for minikube on Ubuntu
# This script sets up a local development environment

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    
    # Check minikube
    if ! command -v minikube &> /dev/null; then
        log_warn "minikube is not installed. Installing..."
        curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
        sudo install minikube-linux-amd64 /usr/local/bin/minikube
        rm minikube-linux-amd64
    fi
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        log_warn "kubectl is not installed. Installing..."
        curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
        sudo install kubectl /usr/local/bin/kubectl
        rm kubectl
    fi
    
    # Check Go
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed. Please install Go 1.22+."
        exit 1
    fi
    
    GO_VERSION=$(go version | grep -oP '\d+\.\d+' | head -1)
    if [[ "${GO_VERSION}" < "1.22" ]]; then
        log_error "Go 1.22+ is required. Current version: ${GO_VERSION}"
        exit 1
    fi
    
    log_info "All prerequisites are met!"
}

# Start minikube
start_minikube() {
    log_info "Starting minikube..."
    
    if minikube status | grep -q "Running"; then
        log_info "minikube is already running"
    else
        minikube start \
            --cpus=4 \
            --memory=8192 \
            --disk-size=50g \
            --driver=docker \
            --kubernetes-version=v1.29.0
    fi
    
    # Enable addons
    log_info "Enabling minikube addons..."
    minikube addons enable metrics-server
    minikube addons enable storage-provisioner
    minikube addons enable default-storageclass
    
    # Wait for cluster to be ready
    log_info "Waiting for cluster to be ready..."
    kubectl wait --for=condition=Ready nodes --all --timeout=120s
}

# Build the operator
build_operator() {
    log_info "Building operator..."
    cd "${PROJECT_ROOT}"
    
    # Download dependencies and generate go.sum
    log_info "Downloading Go dependencies..."
    go mod tidy
    
    # Build the operator binary
    make build
    
    # Build Docker images for minikube
    log_info "Building Docker images in minikube environment..."
    eval $(minikube docker-env)
    
    make docker-build IMG=mssql-operator:dev
    make docker-build-sidecar IMG_SIDECAR=mssql-ag-helper:dev
    
    eval $(minikube docker-env -u)
}

# Install the operator
install_operator() {
    log_info "Installing operator..."
    cd "${PROJECT_ROOT}"
    
    # Install CRDs first
    log_info "Installing CRDs..."
    kubectl apply -f deploy/crds/
    
    # Install namespace, RBAC, and operator deployment
    log_info "Installing operator resources..."
    kubectl apply -f deploy/namespace.yaml
    kubectl apply -f deploy/serviceaccount.yaml
    kubectl apply -f deploy/rbac.yaml
    kubectl apply -f deploy/deployment.yaml
    
    # Wait for operator to be ready
    log_info "Waiting for operator to be ready..."
    kubectl wait --for=condition=Available deployment/mssql-operator \
        -n mssql-system --timeout=120s
    
    log_info "Operator installed successfully!"
}

# Uninstall the operator
uninstall_operator() {
    log_info "Uninstalling operator..."
    cd "${PROJECT_ROOT}"
    
    # Delete deployment, RBAC, serviceaccount, namespace
    kubectl delete -f deploy/deployment.yaml --ignore-not-found=true
    kubectl delete -f deploy/rbac.yaml --ignore-not-found=true
    kubectl delete -f deploy/serviceaccount.yaml --ignore-not-found=true
    kubectl delete -f deploy/namespace.yaml --ignore-not-found=true
    
    # Optionally delete CRDs (commented out to preserve data)
    # kubectl delete -f deploy/crds/ --ignore-not-found=true
    
    log_info "Operator uninstalled. CRDs preserved (delete manually if needed)."
}

# Deploy sample SQL Server
# Usage: deploy_sample [yaml_file]
# Default: samples/sqlserver-2025-standalone.yaml
deploy_sample() {
    local YAML_FILE="${1:-samples/sqlserver-2025-standalone.yaml}"
    
    log_info "Deploying sample SQL Server..."
    cd "${PROJECT_ROOT}"
    
    # Validate YAML file exists
    if [ ! -f "${YAML_FILE}" ]; then
        log_error "YAML file not found: ${YAML_FILE}"
        log_info "Available samples:"
        ls -1 samples/*.yaml
        exit 1
    fi
    
    # Create namespace
    kubectl create namespace mssql --dry-run=client -o yaml | kubectl apply -f -
    
    # Deploy sample
    log_info "Applying configuration from: ${YAML_FILE}"
    kubectl apply -f "${YAML_FILE}"
    
    log_info "Sample SQL Server deployment created!"
    log_info "Watch status with: kubectl get sqlserver -n mssql -w"
}

# Deploy monitoring stack (Prometheus + Grafana)
deploy_monitoring() {
    log_info "Deploying monitoring stack (Prometheus + Grafana)..."
    
    # Deploy Prometheus
    log_info "Deploying Prometheus..."
    kubectl apply -f "${PROJECT_ROOT}/deploy/monitoring/prometheus.yaml"
    
    # Wait for Prometheus to be ready
    log_info "Waiting for Prometheus to be ready..."
    kubectl wait --for=condition=available deployment/prometheus -n monitoring --timeout=120s || true
    
    # Deploy Grafana
    log_info "Deploying Grafana..."
    kubectl apply -f "${PROJECT_ROOT}/deploy/monitoring/grafana.yaml"
    
    # Wait for Grafana to be ready
    log_info "Waiting for Grafana to be ready..."
    kubectl wait --for=condition=available deployment/grafana -n monitoring --timeout=120s || true
    
    log_info "Monitoring stack deployed!"
    echo ""
    log_info "Access Grafana:"
    echo "  kubectl port-forward -n monitoring svc/grafana 3000:3000"
    echo "  Open: http://localhost:3000"
    echo "  Credentials: admin / admin"
    echo ""
    log_info "Access Prometheus:"
    echo "  kubectl port-forward -n monitoring svc/prometheus 9090:9090"
    echo "  Open: http://localhost:9090"
    echo ""
    log_info "For external access with LoadBalancer (minikube):"
    echo "  minikube tunnel"
    echo "  Then: kubectl get svc -n monitoring"
}

# Show status
show_status() {
    log_info "Cluster Status:"
    echo ""
    kubectl get nodes
    echo ""
    
    log_info "Operator Status:"
    kubectl get pods -n mssql-system
    echo ""
    
    log_info "SQL Server Resources:"
    kubectl get sqlserver --all-namespaces
    kubectl get sqlserverag --all-namespaces
    echo ""
    
    log_info "Pods:"
    kubectl get pods -n mssql
    echo ""
    
    # Show monitoring if deployed
    if kubectl get namespace monitoring &>/dev/null; then
        log_info "Monitoring:"
        kubectl get pods -n monitoring
        echo ""
    fi
}

# Cleanup
cleanup() {
    log_info "Cleaning up..."
    
    # Delete sample resources
    kubectl delete -f samples/ --ignore-not-found=true
    
    # Delete operator
    helm uninstall mssql-operator -n mssql-system --ignore-not-found || true
    
    # Delete CRDs
    kubectl delete crd sqlservers.mssql.microsoft.com --ignore-not-found=true
    kubectl delete crd sqlserverags.mssql.microsoft.com --ignore-not-found=true
    kubectl delete crd operatorconfigurations.mssql.microsoft.com --ignore-not-found=true
    
    # Delete namespaces
    kubectl delete namespace mssql --ignore-not-found=true
    kubectl delete namespace mssql-system --ignore-not-found=true
    
    log_info "Cleanup complete!"
}

# Connect to SQL Server
connect_sql() {
    log_info "Connecting to SQL Server..."
    
    # Get the SQL Server pod
    POD=$(kubectl get pods -n mssql -l app=mssql -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    
    if [ -z "$POD" ]; then
        log_error "No SQL Server pods found"
        exit 1
    fi
    
    # Get the password
    PASSWORD=$(kubectl get secret mssql-sa-password -n mssql -o jsonpath='{.data.password}' | base64 -d)
    
    log_info "Connecting to pod: ${POD}"
    log_info "Use password: ${PASSWORD}"
    
    # Port forward
    kubectl port-forward -n mssql pod/${POD} 1433:1433 &
    PF_PID=$!
    
    log_info "Port forwarding started. Connect with:"
    log_info "  sqlcmd -S localhost,1433 -U sa -P '${PASSWORD}'"
    log_info "Press Ctrl+C to stop port forwarding"
    
    wait $PF_PID
}

# Main
main() {
    case "${1:-}" in
        prereq)
            check_prerequisites
            ;;
        start)
            check_prerequisites
            start_minikube
            ;;
        build)
            build_operator
            ;;
        install)
            install_operator
            ;;
        uninstall)
            uninstall_operator
            ;;
        deploy)
            deploy_sample "${2:-}"
            ;;
        monitoring)
            deploy_monitoring
            ;;
        status)
            show_status
            ;;
        cleanup)
            cleanup
            ;;
        connect)
            connect_sql
            ;;
        all)
            check_prerequisites
            start_minikube
            build_operator
            install_operator
            deploy_sample
            show_status
            ;;
        all-with-monitoring)
            check_prerequisites
            start_minikube
            build_operator
            install_operator
            deploy_monitoring
            deploy_sample
            show_status
            ;;
        *)
            echo "Usage: $0 {prereq|start|build|install|uninstall|deploy|monitoring|status|cleanup|connect|all|all-with-monitoring} [options]"
            echo ""
            echo "Commands:"
            echo "  prereq    - Check prerequisites (Docker, minikube, kubectl, Go)"
            echo "  start     - Start minikube cluster"
            echo "  build     - Build operator and sidecar images"
            echo "  install   - Install CRDs and deploy operator"
            echo "  uninstall - Remove operator (preserves CRDs)"
            echo "  deploy [yaml_file] - Deploy SQL Server from YAML file"
            echo "                       Default: samples/sqlserver-2025-standalone.yaml"
            echo "  monitoring - Deploy Prometheus + Grafana monitoring stack"
            echo "  status    - Show cluster and resource status"
            echo "  cleanup   - Remove all resources"
            echo "  connect   - Port-forward to SQL Server"
            echo "  all       - Run all steps (prereq, start, build, install, deploy)"
            echo "  all-with-monitoring - Full setup including Prometheus/Grafana"
            echo ""
            echo "Examples:"
            echo "  $0 all                                       # Full setup with SQL 2025"
            echo "  $0 all-with-monitoring                       # Full setup with monitoring"
            echo "  $0 monitoring                                # Deploy monitoring only"
            echo "  $0 deploy                                    # Deploy SQL 2025 standalone (default)"
            echo "  $0 deploy samples/sqlserver-2022-standalone.yaml"
            echo "  $0 deploy samples/sqlserver-availability-group.yaml"
            exit 1
            ;;
    esac
}

main "$@"
