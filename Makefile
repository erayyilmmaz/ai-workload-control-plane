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
GOVULNCHECK_VERSION := $(shell jq -r '.tools.govulncheck' toolchain.lock.json)
TERRAFORM_VERSION := $(shell jq -r '.tools.terraform' toolchain.lock.json)
TRIVY_VERSION := $(shell jq -r '.tools.trivy' toolchain.lock.json)
ENVTEST_REVISION := $(shell jq -r '.tools.setupEnvtest.revision' toolchain.lock.json)
CONTROLLER_GEN := .tools/bin/controller-gen-$(CONTROLLER_TOOLS_VERSION)/controller-gen
KUSTOMIZE := .tools/bin/kustomize-$(KUSTOMIZE_VERSION)/kustomize
LINT := .tools/bin/golangci-lint-$(LINT_VERSION)/golangci-lint
GOVULNCHECK := .tools/bin/govulncheck-$(GOVULNCHECK_VERSION)/govulncheck
TERRAFORM := .tools/bin/terraform
TRIVY := .tools/bin/trivy
SETUP_ENVTEST := .tools/bin/setup-envtest-$(ENVTEST_REVISION)/setup-envtest
IMG ?= awcp-manager:awcp-15
VERSION ?= 0.1.0-dev
DEPLOY_IMG ?=
DEMO_IMG_V1 ?= awcp-demo:v1
DEMO_IMG_V2 ?= awcp-demo:v2
KIND_CLUSTER ?= awcp-quickstart
GITOPS_REPO_URL ?= https://github.com/erayyilmmaz/ai-workload-control-plane.git
GITOPS_REVISION ?= $(shell git rev-parse HEAD)
REVISION ?= $(shell git rev-parse HEAD)

.PHONY: bootstrap tools infra-tools infra-verify check-go tidy generate manifests fmt fmt-check build vet lint lint-fix test test-unit test-envtest test-race coverage envtest verify verify-generated verify-docs gitops-verify gitops-bootstrap gitops-e2e render docker-build demo-test demo-build smoke e2e portfolio-demo kind kubectl kind-up kind-load kind-down quickstart install deploy undeploy release-bundle verify-package vuln verify-ci
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

infra-tools:
	@test -x "$(TERRAFORM)" -a -x "$(TRIVY)" || bash hack/bootstrap-tools.sh terraform trivy

infra-verify: infra-tools
	bash infra/terraform/verify.sh "$(TERRAFORM)" "$(TRIVY)"

$(CONTROLLER_GEN): toolchain.lock.json | check-go
	GOBIN="$(CURDIR)/$(@D)" $(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)
$(KUSTOMIZE): toolchain.lock.json | check-go
	GOBIN="$(CURDIR)/$(@D)" $(GO) install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)
$(LINT): toolchain.lock.json | check-go
	GOBIN="$(CURDIR)/$(@D)" $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINT_VERSION)
$(GOVULNCHECK): toolchain.lock.json | check-go
	GOBIN="$(CURDIR)/$(@D)" $(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
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
fmt-check: $(LINT)
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
coverage: check-go envtest
	mkdir -p dist
	$(GO) test -count=1 -covermode=atomic -coverpkg="$$($(GO) list ./... | paste -sd, -)" -coverprofile=dist/coverage.out ./...
	$(GO) tool cover -func=dist/coverage.out > dist/coverage.txt
verify: generate manifests build vet lint fmt-check test demo-test verify-generated verify-docs gitops-verify infra-verify render
verify-generated: $(CONTROLLER_GEN)
	bash hack/verify-generated.sh
verify-docs:
	bash test/docs/verify-portfolio.sh
gitops-verify: $(KUSTOMIZE)
	bash test/gitops/verify-gitops.sh
gitops-bootstrap:
	bash gitops/argocd/bootstrap.sh
gitops-e2e: render docker-build demo-build gitops-verify
	bash test/e2e/gitops-e2e.sh "$(GITOPS_REPO_URL)" "$(GITOPS_REVISION)" "$(IMG)" "$(DEMO_IMG_V1)"
render: $(KUSTOMIZE)
	mkdir -p dist
	$(KUSTOMIZE) build config/default > dist/install.yaml
docker-build:
	docker build --build-arg VERSION="$(VERSION)" --build-arg REVISION="$(REVISION)" -t "$(IMG)" .
demo-test: check-go
	cd examples/demo-app && ../../.tools/go/bin/go test -count=1 ./...
demo-build:
	docker build --build-arg VERSION=v1 -t "$(DEMO_IMG_V1)" examples/demo-app
	docker build --build-arg VERSION=v2 -t "$(DEMO_IMG_V2)" examples/demo-app
smoke: render
	bash test/e2e/bootstrap-smoke.sh "$(IMG)"
e2e: render docker-build demo-test demo-build
	bash test/e2e/lifecycle-e2e.sh "$(IMG)" "$(DEMO_IMG_V1)" "$(DEMO_IMG_V2)"
portfolio-demo:
	bash demo/portfolio-demo.sh
kind:
	@test -x .tools/bin/kind || bash hack/bootstrap-tools.sh kind
kubectl:
	@test -x .tools/bin/kubectl || bash hack/bootstrap-tools.sh kubectl
kind-up: kind
	.tools/bin/kind create cluster --name "$(KIND_CLUSTER)" --image "$$(jq -r '.kubernetes.kindNodeImage' toolchain.lock.json)" --wait 120s
kind-load: kind
	docker image inspect "$(IMG)" "$(DEMO_IMG_V1)" "$(DEMO_IMG_V2)" >/dev/null
	.tools/bin/kind load docker-image "$(IMG)" "$(DEMO_IMG_V1)" "$(DEMO_IMG_V2)" --name "$(KIND_CLUSTER)"
kind-down: kind
	.tools/bin/kind delete cluster --name "$(KIND_CLUSTER)"
quickstart: kind-up
	$(MAKE) IMG=awcp-manager:quickstart docker-build
	$(MAKE) demo-build
	$(MAKE) IMG=awcp-manager:quickstart kind-load
	$(MAKE) DEPLOY_IMG=awcp-manager:quickstart deploy
	.tools/bin/kubectl apply -f examples/basic.yaml
	.tools/bin/kubectl -n awcp-workloads wait --for=condition=Ready aiworkload/basic-demo --timeout=180s
install: kubectl
	.tools/bin/kubectl apply -k config/install
deploy: kubectl
	@test -n "$(DEPLOY_IMG)" || { echo 'Set DEPLOY_IMG to an accessible immutable image reference, preferably repo@sha256:...'; exit 1; }
	root_dir="$(CURDIR)"; task_dir="$$(mktemp -d "$${TMPDIR:-/tmp}/awcp-deploy.XXXXXX")"; trap 'rm -rf "$$task_dir"' EXIT; cp -R config "$$task_dir/config"; cd "$$task_dir/config/default"; "$$root_dir/$(KUSTOMIZE)" edit set image "awcp-manager=$(DEPLOY_IMG)"; "$$root_dir/.tools/bin/kubectl" apply -k .; "$$root_dir/.tools/bin/kubectl" wait --for=condition=Established crd/aiworkloads.platform.example.io --timeout=60s; "$$root_dir/.tools/bin/kubectl" -n awcp-system rollout status deployment/awcp-controller-manager --timeout=180s
undeploy: kubectl
	.tools/bin/kubectl delete -k config/uninstall --ignore-not-found
release-bundle: render
	mkdir -p dist/release
	$(KUSTOMIZE) build config/install > dist/release/awcp-crds.yaml
	$(KUSTOMIZE) build config/default > dist/release/awcp-operator.yaml
	$(KUSTOMIZE) build config/uninstall > dist/release/awcp-operator-uninstall.yaml
	shasum -a 256 dist/release/awcp-crds.yaml dist/release/awcp-operator.yaml dist/release/awcp-operator-uninstall.yaml > dist/release/SHA256SUMS
verify-package:
	bash test/packaging/verify-bundle.sh
vuln: $(GOVULNCHECK)
	$(GOVULNCHECK) ./...
verify-ci:
	bash test/ci/verify-workflow.sh
