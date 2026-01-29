# MSSQL Kubernetes Operator Makefile

# Version configuration
VERSION ?= 1.0.0

# Registry configuration - update for your environment
REGISTRY ?= localhost:5000/sqlserver-operator

# Image URL to use all building/pushing image targets
IMG ?= $(REGISTRY)/operator:$(VERSION)
SIDECAR_IMG ?= $(REGISTRY)/ag-helper:$(VERSION)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: tidy
tidy: ## Run go mod tidy to download dependencies and update go.sum
	go mod tidy

.PHONY: manifests
manifests: controller-gen tidy ## Generate CRD manifests.
	$(CONTROLLER_GEN) crd paths="./pkg/apis/mssql.microsoft.com/v1alpha1" output:crd:dir=./deploy/crds
	@echo "CRDs generated in deploy/crds/"

.PHONY: generate-install-yaml
generate-install-yaml: ## Generate the combined install.yaml for direct URL installation.
	@echo "Generating install.yaml..."
	@./scripts/generate-install-yaml.sh $(VERSION) $(IMG)

.PHONY: generate
generate: controller-gen tidy ## Generate code (DeepCopy functions).
	$(CONTROLLER_GEN) object paths="./pkg/apis/mssql.microsoft.com/v1alpha1"
	@echo "DeepCopy methods generated"

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet ## Run tests.
	go test ./... -coverprofile cover.out

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

##@ Build

.PHONY: build
build: generate fmt vet ## Build operator binary.
	go build -o bin/mssql-operator ./cmd/mssql-operator

.PHONY: build-sidecar
build-sidecar: ## Build AG helper sidecar binary.
	go build -o bin/mssql-ag-helper ./cmd/ag-helper

.PHONY: run
run: manifests generate fmt vet ## Run operator from your host.
	go run ./cmd/mssql-operator

.PHONY: docker-build
docker-build: ## Build docker image with the operator.
	docker build -t ${IMG} -f Dockerfile.operator .

.PHONY: docker-build-sidecar
docker-build-sidecar: ## Build docker image with the AG helper sidecar.
	docker build -t ${SIDECAR_IMG} -f Dockerfile.ag-helper .

.PHONY: docker-push
docker-push: ## Push docker image with the operator.
	docker push ${IMG}

.PHONY: docker-push-sidecar
docker-push-sidecar: ## Push docker image with the AG helper sidecar.
	docker push ${SIDECAR_IMG}

##@ Deployment

.PHONY: install-crds
install-crds: ## Install CRDs into the K8s cluster.
	kubectl apply -f deploy/crds/

.PHONY: uninstall-crds
uninstall-crds: ## Uninstall CRDs from the K8s cluster.
	kubectl delete -f deploy/crds/ --ignore-not-found=true

.PHONY: deploy
deploy: install-crds ## Deploy operator to the K8s cluster.
	kubectl apply -f deploy/namespace.yaml
	kubectl apply -f deploy/serviceaccount.yaml
	kubectl apply -f deploy/rbac.yaml
	kubectl apply -f deploy/deployment.yaml

.PHONY: undeploy
undeploy: ## Undeploy operator from the K8s cluster.
	kubectl delete -f deploy/deployment.yaml --ignore-not-found=true
	kubectl delete -f deploy/rbac.yaml --ignore-not-found=true
	kubectl delete -f deploy/serviceaccount.yaml --ignore-not-found=true
	kubectl delete -f deploy/namespace.yaml --ignore-not-found=true

##@ Local Development (Minikube)

.PHONY: minikube-setup
minikube-setup: ## Setup minikube for local development.
	./scripts/dev-setup.sh prereq
	./scripts/dev-setup.sh start

.PHONY: minikube-deploy
minikube-deploy: docker-build docker-build-sidecar ## Build and deploy to minikube.
	eval $$(minikube docker-env) && \
	docker build -t ${IMG} -f Dockerfile . && \
	docker build -t ${SIDECAR_IMG} -f Dockerfile.sidecar . && \
	$(MAKE) deploy

##@ Tools

CONTROLLER_GEN = $(GOBIN)/controller-gen
.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	@test -s $(CONTROLLER_GEN) || go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

GOLANGCI_LINT = $(GOBIN)/golangci-lint
.PHONY: golangci-lint
golangci-lint: ## Download golangci-lint locally if necessary.
	@test -s $(GOLANGCI_LINT) || go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2

##@ Distribution & Packaging

.PHONY: docker-build-all
docker-build-all: ## Build all Docker images.
	docker build -t $(IMG) -f Dockerfile.operator .
	docker build -t $(SIDECAR_IMG) -f Dockerfile.ag-helper .
	@echo ""
	@echo "Images built:"
	@echo "  $(IMG)"
	@echo "  $(SIDECAR_IMG)"

.PHONY: docker-push-all
docker-push-all: ## Push all Docker images to registry.
	docker push $(IMG)
	docker push $(SIDECAR_IMG)
	docker tag $(IMG) $(REGISTRY)/operator:latest
	docker tag $(SIDECAR_IMG) $(REGISTRY)/ag-helper:latest
	docker push $(REGISTRY)/operator:latest
	docker push $(REGISTRY)/ag-helper:latest
	@echo ""
	@echo "Images pushed to $(REGISTRY)"

.PHONY: release
release: manifests generate-install-yaml docker-build-all docker-push-all ## Build, push images and generate manifests for release.
	@echo ""
	@echo "Release $(VERSION) complete!"
	@echo "Images: $(REGISTRY)/operator:$(VERSION), $(REGISTRY)/ag-helper:$(VERSION)"
	@echo "Install manifest: install.yaml"

.PHONY: dist
dist: manifests generate-install-yaml ## Create distribution package for sharing.
	@echo "Creating distribution package v$(VERSION)..."
	@mkdir -p dist/sqlserver-operator-$(VERSION)
	@cp -r deploy/ dist/sqlserver-operator-$(VERSION)/
	@cp -r samples/ dist/sqlserver-operator-$(VERSION)/
	@cp README.md dist/sqlserver-operator-$(VERSION)/
	@test -f LICENSE && cp LICENSE dist/sqlserver-operator-$(VERSION)/ || true
	@echo '#!/bin/bash' > dist/sqlserver-operator-$(VERSION)/install.sh
	@echo 'set -e' >> dist/sqlserver-operator-$(VERSION)/install.sh
	@echo 'echo "Installing SQL Server Kubernetes Operator v$(VERSION)..."' >> dist/sqlserver-operator-$(VERSION)/install.sh
	@echo 'kubectl create namespace mssql-operator --dry-run=client -o yaml | kubectl apply -f -' >> dist/sqlserver-operator-$(VERSION)/install.sh
	@echo 'kubectl create namespace mssql --dry-run=client -o yaml | kubectl apply -f -' >> dist/sqlserver-operator-$(VERSION)/install.sh
	@echo 'kubectl apply -f deploy/crds/' >> dist/sqlserver-operator-$(VERSION)/install.sh
	@echo 'kubectl apply -f deploy/rbac.yaml' >> dist/sqlserver-operator-$(VERSION)/install.sh
	@echo 'kubectl apply -f deploy/serviceaccount.yaml' >> dist/sqlserver-operator-$(VERSION)/install.sh
	@echo 'kubectl apply -f deploy/operator.yaml' >> dist/sqlserver-operator-$(VERSION)/install.sh
	@echo 'echo "Waiting for operator to be ready..."' >> dist/sqlserver-operator-$(VERSION)/install.sh
	@echo 'kubectl wait --for=condition=available --timeout=120s deployment/mssql-operator -n mssql-operator' >> dist/sqlserver-operator-$(VERSION)/install.sh
	@echo 'echo ""' >> dist/sqlserver-operator-$(VERSION)/install.sh
	@echo 'echo "✅ SQL Server Operator installed successfully!"' >> dist/sqlserver-operator-$(VERSION)/install.sh
	@echo 'echo ""' >> dist/sqlserver-operator-$(VERSION)/install.sh
	@echo 'echo "Next steps:"' >> dist/sqlserver-operator-$(VERSION)/install.sh
	@echo 'echo "  kubectl create secret generic sql-sa-secret --from-literal=password=YourStrong@Passw0rd! -n mssql"' >> dist/sqlserver-operator-$(VERSION)/install.sh
	@echo 'echo "  kubectl apply -f samples/sqlserver-2025-standalone.yaml"' >> dist/sqlserver-operator-$(VERSION)/install.sh
	@chmod +x dist/sqlserver-operator-$(VERSION)/install.sh
	@echo '#!/bin/bash' > dist/sqlserver-operator-$(VERSION)/uninstall.sh
	@echo 'set -e' >> dist/sqlserver-operator-$(VERSION)/uninstall.sh
	@echo 'echo "Uninstalling SQL Server Kubernetes Operator..."' >> dist/sqlserver-operator-$(VERSION)/uninstall.sh
	@echo 'kubectl delete -f deploy/operator.yaml --ignore-not-found' >> dist/sqlserver-operator-$(VERSION)/uninstall.sh
	@echo 'kubectl delete -f deploy/rbac.yaml --ignore-not-found' >> dist/sqlserver-operator-$(VERSION)/uninstall.sh
	@echo 'kubectl delete -f deploy/serviceaccount.yaml --ignore-not-found' >> dist/sqlserver-operator-$(VERSION)/uninstall.sh
	@echo 'echo "✅ Operator uninstalled (CRDs preserved)"' >> dist/sqlserver-operator-$(VERSION)/uninstall.sh
	@chmod +x dist/sqlserver-operator-$(VERSION)/uninstall.sh
	@cd dist && tar -czvf sqlserver-operator-$(VERSION).tar.gz sqlserver-operator-$(VERSION)/
	@echo ""
	@echo "=========================================="
	@echo "Distribution package created:"
	@echo "  dist/sqlserver-operator-$(VERSION).tar.gz"
	@echo ""
	@echo "To share with others:"
	@echo "  1. Share the .tar.gz file"
	@echo "  2. Recipients run: tar -xzf sqlserver-operator-$(VERSION).tar.gz"
	@echo "  3. Recipients run: cd sqlserver-operator-$(VERSION) && ./install.sh"
	@echo "=========================================="

.PHONY: clean
clean: ## Clean build artifacts and distribution packages.
	rm -rf bin/
	rm -rf dist/
	rm -f cover.out
	@echo "Cleaned build artifacts"
	@echo "Note: install.yaml is preserved (commit it to enable direct URL installation)"

.PHONY: version
version: ## Display the current version.
	@echo "Version: $(VERSION)"
	@echo "Operator Image: $(IMG)"
	@echo "AG Helper Image: $(SIDECAR_IMG)"

