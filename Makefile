# Image URL to use all building/pushing image targets
REGISTRY ?= docker.imgdb.de/deeplythink
IMG ?= custom-scheduler
TAG ?= latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
GOBIN=$(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN=$(shell go env GOPATH)/bin
endif

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION=1.29.0

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: fmt vet ## Build scheduler binary.
	go build -o bin/scheduler cmd/scheduler/main.go

.PHONY: run
run: fmt vet ## Run scheduler from your host.
	go run ./cmd/scheduler/main.go

.PHONY: docker-build
docker-build: ## Build docker image with the scheduler.
	docker build -t $(REGISTRY)/$(IMG):$(TAG) -f deploy/docker/Dockerfile .

.PHONY: docker-push
docker-push: ## Push docker image with the scheduler.
	docker push $(REGISTRY)/$(IMG):$(TAG)

##@ Deployment

.PHONY: deploy
deploy: ## Deploy scheduler to the K8s cluster specified in ~/.kube/config.
	kubectl apply -f deploy/kubernetes/scheduler-deployment.yaml

.PHONY: undeploy
undeploy: ## Undeploy scheduler from the K8s cluster specified in ~/.kube/config.
	kubectl delete -f deploy/kubernetes/scheduler-deployment.yaml

.PHONY: deploy-test-pod
deploy-test-pod: ## Deploy test pod using custom scheduler
	kubectl apply -f examples/test-pod.yaml

.PHONY: redeploy
redeploy: undeploy deploy ## Redeploy scheduler (undeploy + deploy)

.PHONY: build-deploy
build-deploy: docker-build docker-push deploy ## Build, push and deploy

##@ Clean

.PHONY: clean
clean: ## Clean build files
	rm -rf bin/

##@ Debug

.PHONY: logs
logs: ## View scheduler logs
	kubectl logs -n kube-system -l app=custom-scheduler -f

.PHONY: status
status: ## Check scheduler status
	@echo "=== Scheduler Pod Status ==="
	@kubectl get pods -n kube-system -l app=custom-scheduler
	@echo "\n=== Scheduler Events ==="
	@kubectl get events -n kube-system --field-selector involvedObject.kind=Pod,involvedObject.name=custom-scheduler

##@ Configuration

.PHONY: show-config
show-config: ## Show current scheduler configuration
	@echo "=== Scheduler ConfigMap ==="
	@kubectl get configmap -n kube-system scheduler-config -o yaml
	@echo "\n=== Local Path ConfigMap ==="
	@kubectl get configmap -n kube-system local-path-config -o yaml

# Build the docker image
docker-build-local: ## Build the docker image locally
	docker build -t $(IMG):$(TAG) -f deploy/docker/Dockerfile .

# Push the docker image
docker-push-local: ## Push the docker image to local registry
	docker push $(IMG):$(TAG)

# Example usage:
# make docker-build REGISTRY=your-registry IMG=custom-scheduler TAG=v1.0.0
# make docker-push REGISTRY=your-registry IMG=custom-scheduler TAG=v1.0.0
# make deploy
# make logs
# make status
# make show-config
