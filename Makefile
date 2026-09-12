SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := build

# Relative target names deliberately support checkout paths containing spaces.
export PATH := $(CURDIR)/.tools/go/bin:$(CURDIR)/.tools/bin:$(PATH)
export GOPATH := $(CURDIR)/.tools/gopath
export GOCACHE := $(CURDIR)/.tools/cache/go-build
export GOLANGCI_LINT_CACHE := $(CURDIR)/.tools/cache/golangci-lint
export GOTOOLCHAIN := local
export KUBEBUILDER_ASSETS := $(CURDIR)/.tools/envtest
GO := .tools/go/bin/go
CONTROLLER_TOOLS_VERSION := $(shell jq -r '.tools.controllerTools' toolchain.lock.json)
KUSTOMIZE_VERSION := $(shell jq -r '.tools.kustomize' toolchain.lock.json)
LINT_VERSION := $(shell jq -r '.tools.golangciLint' toolchain.lock.json)
ENVTEST_REVISION := $(shell jq -r '.tools.setupEnvtest.revision' toolchain.lock.json)
CONTROLLER_GEN := .tools/bin/controller-gen-$(CONTROLLER_TOOLS_VERSION)/controller-gen
KUSTOMIZE := .tools/bin/kustomize-$(KUSTOMIZE_VERSION)/kustomize
LINT := .tools/bin/golangci-lint-$(LINT_VERSION)/golangci-lint
SETUP_ENVTEST := .tools/bin/setup-envtest-$(ENVTEST_REVISION)/setup-envtest
IMG ?= awcp-manager:awcp-3
REVISION ?= $(shell git rev-parse HEAD)

.PHONY: bootstrap tools check-go tidy generate manifests fmt build vet lint lint-fix test test-unit test-envtest test-race envtest verify verify-generated render docker-build smoke
bootstrap:
	bash hack/bootstrap-tools.sh
	$(MAKE) tools
	$(GO) mod download

check-go:
	@test "$$($(GO) env GOVERSION)" = "go$$(jq -r '.go.toolchain' toolchain.lock.json)" || { echo 'Run make bootstrap to install the pinned Go toolchain'; exit 1; }

# Keep all selected Kubernetes release-family modules on the same patch even
# when lazy module loading means a member is not compiled by this scaffold.
tidy: check-go
	$(GO) mod tidy
	jq -r '.kubernetes.clientModules | to_entries[] | "\(.key)@\(.value)"' toolchain.lock.json | while read -r pin; do $(GO) mod edit "-require=$$pin"; done
	$(GO) mod download

tools: check-go $(CONTROLLER_GEN) $(KUSTOMIZE) $(LINT) $(SETUP_ENVTEST)

$(CONTROLLER_GEN): toolchain.lock.json | check-go
	GOBIN="$(CURDIR)/$(@D)" $(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)
$(KUSTOMIZE): toolchain.lock.json | check-go
	GOBIN="$(CURDIR)/$(@D)" $(GO) install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)
$(LINT): toolchain.lock.json | check-go
	GOBIN="$(CURDIR)/$(@D)" $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINT_VERSION)
$(SETUP_ENVTEST): toolchain.lock.json | check-go
	GOBIN="$(CURDIR)/$(@D)" $(GO) install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_REVISION)

generate: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) object:headerFile=hack/boilerplate.go.txt paths="./api/..."
manifests: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) rbac:roleName=awcp-manager-role crd paths="./api/...;./internal/controller/..." output:crd:artifacts:config=config/crd/bases
fmt: check-go
	$(GO) fmt ./...
build: check-go
	$(GO) build -mod=readonly -trimpath -o bin/manager ./cmd
vet: check-go
	$(GO) vet ./...
lint: $(LINT)
	$(LINT) run ./...
	$(LINT) fmt --diff
lint-fix: $(LINT)
	$(LINT) fmt
	$(LINT) run --fix ./...
envtest:
	@test -x .tools/envtest/kube-apiserver -a -x .tools/envtest/etcd || bash hack/bootstrap-tools.sh envtest
test: check-go envtest
	$(GO) test -count=1 ./...
test-unit: check-go
	$(GO) test -count=1 -short ./...
test-envtest: check-go envtest
	$(GO) test -count=1 -v ./test/integration/... ./test/contract/... ./test/reconciliation/...
test-race: check-go envtest
	$(GO) test -race -count=1 ./...
verify: generate manifests build vet lint test verify-generated render
verify-generated: $(CONTROLLER_GEN)
	bash hack/verify-generated.sh
render: $(KUSTOMIZE)
	mkdir -p dist
	$(KUSTOMIZE) build config/default > dist/install.yaml
docker-build:
	docker build --build-arg REVISION="$(REVISION)" -t "$(IMG)" .
smoke: render
	bash test/e2e/bootstrap-smoke.sh "$(IMG)"
