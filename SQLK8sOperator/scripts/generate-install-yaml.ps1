# =============================================================================
# Generate install.yaml - Combines all deployment manifests into a single file
# =============================================================================
# This script generates a single install.yaml file that can be applied directly
# from a URL for easy operator installation.
#
# Usage:
#   .\scripts\generate-install-yaml.ps1 [-Version "v1.0.0"] [-Image "ghcr.io/org/mssql-operator:v1.0.0"]
#
# Examples:
#   .\scripts\generate-install-yaml.ps1
#   .\scripts\generate-install-yaml.ps1 -Version "v1.0.0"
#   .\scripts\generate-install-yaml.ps1 -Version "v1.0.0" -Image "ghcr.io/myorg/mssql-operator:v1.0.0"
# =============================================================================

param(
    [string]$Version = "v1.0.0",
    [string]$Image = ""
)

$ErrorActionPreference = "Stop"

# Configuration
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir
$OutputFile = Join-Path $RootDir "install.yaml"
$DeployDir = Join-Path $RootDir "deploy"
$CrdDir = Join-Path $DeployDir "crds"

if ([string]::IsNullOrEmpty($Image)) {
    $Image = "ghcr.io/tejasaks/mssql-operator:$Version"
}

Write-Host "Generating install.yaml..." -ForegroundColor Cyan
Write-Host "  Version: $Version"
Write-Host "  Image: $Image"
Write-Host "  Output: $OutputFile"
Write-Host ""

# Start with header
$timestamp = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$header = @"
# =============================================================================
# SQL Server Kubernetes Operator - Combined Installation Manifest
# =============================================================================
# Version: $Version
# Generated: $timestamp
#
# This file contains all resources needed to install the SQL Server Kubernetes
# Operator. It is auto-generated from the deploy/ directory.
#
# INSTALLATION:
#   kubectl apply -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml
#
# Or download and apply:
#   Invoke-WebRequest -Uri "https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml" -OutFile "install.yaml"
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
"@

Set-Content -Path $OutputFile -Value $header -Encoding UTF8

# Function to add a file with separator
function Add-Manifest {
    param(
        [string]$FilePath,
        [string]$Description
    )
    
    if (Test-Path $FilePath) {
        $relativePath = $FilePath.Replace($RootDir, "").TrimStart("\", "/")
        $separator = @"

# -----------------------------------------------------------------------------
# $Description
# Source: $relativePath
# -----------------------------------------------------------------------------
"@
        Add-Content -Path $OutputFile -Value $separator -Encoding UTF8
        # Use -Raw to preserve exact line content (avoids pipeline line-merging issues)
        $content = Get-Content -Path $FilePath -Raw
        Add-Content -Path $OutputFile -Value $content -NoNewline -Encoding UTF8
        Write-Host "  Added: $relativePath" -ForegroundColor Green
    }
    else {
        Write-Host "  WARNING: File not found: $FilePath" -ForegroundColor Yellow
    }
}

# Add namespaces first
Add-Manifest -FilePath (Join-Path $DeployDir "namespace.yaml") -Description "Namespaces"

# Add service account
Add-Manifest -FilePath (Join-Path $DeployDir "serviceaccount.yaml") -Description "Service Account"

# Add RBAC
Add-Manifest -FilePath (Join-Path $DeployDir "rbac.yaml") -Description "RBAC (ClusterRole and ClusterRoleBinding)"

# Add CRDs
Write-Host ""
Write-Host "Adding CRDs..." -ForegroundColor Cyan
$crdFiles = Get-ChildItem -Path $CrdDir -Filter "*.yaml" -ErrorAction SilentlyContinue
foreach ($crdFile in $crdFiles) {
    Add-Manifest -FilePath $crdFile.FullName -Description "CRD: $($crdFile.Name)"
}

# Add webhook if exists
$webhookFile = Join-Path $DeployDir "webhook.yaml"
if (Test-Path $webhookFile) {
    Add-Manifest -FilePath $webhookFile -Description "Webhook Configuration"
}

# Add deployment last (with image substitution)
Write-Host ""
Write-Host "Adding deployment with image substitution..." -ForegroundColor Cyan
$deploymentFile = Join-Path $DeployDir "deployment.yaml"
if (Test-Path $deploymentFile) {
    $separator = @"

# -----------------------------------------------------------------------------
# Operator Deployment
# Source: deploy/deployment.yaml
# -----------------------------------------------------------------------------
"@
    Add-Content -Path $OutputFile -Value $separator -Encoding UTF8
    
    # Read and substitute image and pull policy for remote installation
    $content = Get-Content -Path $deploymentFile -Raw
    $content = $content -replace 'image:.*mssql-operator.*', "image: $Image"
    # Override imagePullPolicy from Never (local dev) to IfNotPresent (remote install)
    $content = $content -replace 'imagePullPolicy:\s*Never', 'imagePullPolicy: IfNotPresent'
    Add-Content -Path $OutputFile -Value $content -Encoding UTF8
    Write-Host "  Added: deploy/deployment.yaml (with image: $Image)" -ForegroundColor Green
}
else {
    Write-Host "  ERROR: deployment.yaml not found!" -ForegroundColor Red
    exit 1
}

# Add footer
$footer = @"

# ==============================================================================
# End of install.yaml
# ==============================================================================
"@
Add-Content -Path $OutputFile -Value $footer -Encoding UTF8

# Calculate stats
$lineCount = (Get-Content $OutputFile).Count
$fileSize = [math]::Round((Get-Item $OutputFile).Length / 1KB, 2)

Write-Host ""
Write-Host "==========================================" -ForegroundColor Green
Write-Host "install.yaml generated successfully!" -ForegroundColor Green
Write-Host "  Lines: $lineCount"
Write-Host "  Size: ${fileSize} KB"
Write-Host "  Path: $OutputFile"
Write-Host ""
Write-Host "To install the operator:"
Write-Host "  kubectl apply -f install.yaml"
Write-Host ""
Write-Host "Or directly from GitHub:"
Write-Host "  kubectl apply -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml"
Write-Host "==========================================" -ForegroundColor Green
