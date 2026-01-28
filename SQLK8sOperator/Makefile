# MSSQL Kubernetes Operator Makefile

# Image URL to use all building/pushing image targets
IMG ?= mssql-operator:latest
SIDECAR_IMG ?= mssql-ag-helper:latest

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
	$(CONTROLLER_GEN) crd paths="./pkg/apis/..." output:crd:artifacts:config=config/crd/bases
	$(CONTROLLER_GEN) rbac:roleName=mssql-operator paths="./internal/controller/..." output:rbac:artifacts:config=config/rbac

.PHONY: generate
generate: controller-gen tidy ## Generate code (DeepCopy functions).
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./pkg/apis/..."

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
	docker build -t ${IMG} -f Dockerfile .

.PHONY: docker-build-sidecar
docker-build-sidecar: ## Build docker image with the AG helper sidecar.
	docker build -t ${SIDECAR_IMG} -f Dockerfile.sidecar .

.PHONY: docker-push
docker-push: ## Push docker image with the operator.
	docker push ${IMG}

.PHONY: docker-push-sidecar
docker-push-sidecar: ## Push docker image with the AG helper sidecar.
	docker push ${SIDECAR_IMG}

##@ Deployment

.PHONY: install
install: manifests ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kubectl apply -f config/crd/bases

.PHONY: uninstall
uninstall: manifests ## Uninstall CRDs from the K8s cluster.
	kubectl delete -f config/crd/bases

.PHONY: deploy
deploy: manifests ## Deploy operator to the K8s cluster.
	helm upgrade --install mssql-operator ./helm/mssql-operator

.PHONY: undeploy
undeploy: ## Undeploy operator from the K8s cluster.
	helm uninstall mssql-operator

##@ Local Development (Minikube)

.PHONY: minikube-setup
minikube-setup: ## Setup minikube for local development.
	./hack/setup-minikube.sh

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
	@test -s $(CONTROLLER_GEN) || go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0

GOLANGCI_LINT = $(GOBIN)/golangci-lint
.PHONY: golangci-lint
golangci-lint: ## Download golangci-lint locally if necessary.
	@test -s $(GOLANGCI_LINT) || go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2
