SHELL := /bin/bash
.SHELLFLAGS := -euo pipefail -c
.NOTPARALLEL:
.DEFAULT_GOAL := help

LEVEL ?= 5
CLUSTER ?= sre-reference
NAMESPACE ?= replica-system
RELEASE ?= replica-control
IMAGE_REPO ?= replica-control
IMAGE_TAG ?= dev
IMAGE := $(IMAGE_REPO):$(IMAGE_TAG)
include toolchain.env
BIN := $(CURDIR)/.bin
TOOLS := $(CURDIR)/.tools/bin
CHART := charts/replica-control
CORE := ./internal/model ./internal/service ./internal/httpapi ./internal/reconcile ./internal/security ./internal/health ./cmd/certgen

.PHONY: help doctor generate prepare test test-core vet build certs cluster docker-build deploy integration integration-all upgrade-test helm-check clean clean-cluster
help:
	@printf '%s\n' 'make test-core                 Offline, dependency-free tests with race detection' 'make prepare                   Generate protobuf Go bindings and resolve go.sum' 'make test                      Full unit tests (requires dependencies)' 'make deploy LEVEL=1..5          Build image and deploy into a named local KIND cluster' 'make integration LEVEL=1..5     Exercise that level against KIND' 'make integration-all           Exercise each level in sequence' 'make upgrade-test LEVEL=3|4|5     Probe the real Service while rolling the deployment' 'make clean-cluster             Delete only the named development KIND cluster'

doctor:
	@for tool in go make docker kubectl kind helm protoc curl jq; do command -v "$$tool" >/dev/null || { echo "Missing required tool: $$tool. See README.md." >&2; exit 1; }; done
	@docker info >/dev/null
	@go version; protoc --version; helm version --short; kind version

generate:
	@command -v protoc >/dev/null || { echo 'Install protoc (see README.md).' >&2; exit 1; }
	@mkdir -p "$(TOOLS)" gen
	GOBIN="$(TOOLS)" go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOBUF_VERSION)
	GOBIN="$(TOOLS)" go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(GRPC_PLUGIN_VERSION)
	PATH="$(TOOLS):$$PATH" protoc -I api --go_out=gen --go_opt=paths=source_relative --go-grpc_out=gen --go-grpc_opt=paths=source_relative api/replicas/v1/replicas.proto

prepare: generate
	go mod tidy
	go mod verify

# The delivered environment had no module downloads. This target remains usable
# with Go 1.23+ and verifies ONLY the explicitly listed dependency-free packages.
test-core:
	GOTOOLCHAIN=local GOPROXY=off go test -modfile=go.offline.mod -race -count=1 $(CORE)

test: check-prepared
	go test -mod=readonly -race -count=1 ./...

vet: check-prepared
	go vet -mod=readonly ./...

build: check-prepared
	@mkdir -p "$(BIN)"
	go build -mod=readonly -trimpath -ldflags='-X main.version=$(IMAGE_TAG)' -o "$(BIN)/server" ./cmd/server
	go build -mod=readonly -trimpath -o "$(BIN)/replicactl" ./cmd/replicactl
	go build -mod=readonly -trimpath -o "$(BIN)/probe" ./cmd/probe

certs:
	@mkdir -p .local
	@if [ -f .local/pki/server.crt ] && [ -f .local/pki/server.key ] && [ -f .local/pki/client.crt ] && [ -f .local/pki/client.key ] && [ -f .local/pki/ca.crt ]; then \
	  echo 'Reusing .local/pki (certificates are not silently rotated).'; \
	else \
	  GOTOOLCHAIN=local go run -modfile=go.offline.mod ./cmd/certgen --dns 'localhost,$(RELEASE),$(RELEASE).$(NAMESPACE).svc,$(RELEASE).$(NAMESPACE).svc.cluster.local'; \
	fi

cluster:
	@if ! kind get clusters | grep -Fxq '$(CLUSTER)'; then kind create cluster --name '$(CLUSTER)' --image '$(KIND_NODE_IMAGE)' --wait 120s; fi

# make prepare must have produced the actual dependency lock and protobuf bindings.
docker-build: check-prepared
	docker build --build-arg GO_IMAGE='$(GO_IMAGE)' --build-arg VERSION='$(IMAGE_TAG)' -t '$(IMAGE)' .

deploy: doctor docker-build cluster certs
	kind load docker-image '$(IMAGE)' --name '$(CLUSTER)'
	kubectl --context 'kind-$(CLUSTER)' create namespace '$(NAMESPACE)' --dry-run=client -o yaml | kubectl --context 'kind-$(CLUSTER)' apply -f -
	kubectl --context 'kind-$(CLUSTER)' apply -f $(CHART)/crds/
	kubectl --context 'kind-$(CLUSTER)' wait --for=condition=Established crd/replicaintents.replicas.reference.example.com --timeout=60s
	kubectl --context 'kind-$(CLUSTER)' -n '$(NAMESPACE)' create secret generic '$(RELEASE)-tls' --from-file=tls.crt=.local/pki/server.crt --from-file=tls.key=.local/pki/server.key --from-file=ca.crt=.local/pki/ca.crt --dry-run=client -o yaml | kubectl --context 'kind-$(CLUSTER)' apply -f -
	helm upgrade --install '$(RELEASE)' $(CHART) --kube-context 'kind-$(CLUSTER)' -n '$(NAMESPACE)' --set level=$(LEVEL) --set image.repository='$(IMAGE_REPO)' --set-string image.tag='$(IMAGE_TAG)' --set tls.existingSecret='$(RELEASE)-tls' --wait=watcher --rollback-on-failure --timeout 180s

integration: deploy build
	LEVEL='$(LEVEL)' CLUSTER='$(CLUSTER)' NAMESPACE='$(NAMESPACE)' RELEASE='$(RELEASE)' IMAGE='$(IMAGE)' bash scripts/integration.sh

integration-all:
	@for level in 1 2 3 4 5; do $(MAKE) integration LEVEL=$$level; done

upgrade-test: deploy build
	@test '$(LEVEL)' = 3 -o '$(LEVEL)' = 4 -o '$(LEVEL)' = 5 || { echo 'Use LEVEL=3, LEVEL=4 or LEVEL=5.'; exit 1; }
	UPGRADE_TEST=1 LEVEL='$(LEVEL)' CLUSTER='$(CLUSTER)' NAMESPACE='$(NAMESPACE)' RELEASE='$(RELEASE)' IMAGE='$(IMAGE)' bash scripts/integration.sh

helm-check:
	helm lint $(CHART)
	@mkdir -p artifacts
	@for level in 1 2 3 4 5; do helm template '$(RELEASE)' $(CHART) -n '$(NAMESPACE)' --include-crds --set level=$$level > artifacts/level-$$level.yaml; done

clean:
	rm -rf .bin .tools artifacts coverage.out

# Destructive only to the explicitly named LOCAL development cluster. PKI is kept.
clean-cluster:
	kind delete cluster --name '$(CLUSTER)'

.PHONY: check-prepared format format-check verify-generated workflow-check quality vuln outdated pull-images docker-test package
check-prepared:
	@test -s go.sum -a -s gen/replicas/v1/replicas.pb.go -a -s gen/replicas/v1/replicas_grpc.pb.go || { echo 'Run make prepare first, then review go.mod, go.sum and gen/.' >&2; exit 1; }

format:
	gofmt -w cmd internal gen

format-check:
	bash scripts/check-format.sh

verify-generated: check-prepared
	bash scripts/verify-generated.sh

workflow-check:
	actionlint -shellcheck= -pyflakes=

quality: toolchain-check format-check verify-generated test vet build helm-check workflow-check
	go mod verify

vuln: check-prepared
	govulncheck ./...

outdated: check-prepared
	go list -m -u -mod=readonly all

pull-images:
	docker pull '$(GO_IMAGE)'
	docker pull '$(KIND_NODE_IMAGE)'
	docker pull '$(PAUSE_IMAGE)'

docker-test: check-prepared
	docker build --target test --build-arg GO_IMAGE='$(GO_IMAGE)' -t '$(IMAGE_REPO):test' .

package: check-prepared
	@mkdir -p artifacts
	helm package $(CHART) --destination artifacts
	docker image save '$(IMAGE)' | gzip > artifacts/replica-control-image.tar.gz

.PHONY: toolchain-check
toolchain-check:
	python3 scripts/check-toolchain.py
