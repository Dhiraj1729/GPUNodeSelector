SHELL := /bin/bash
.DEFAULT_GOAL := help

.PHONY: help tools doctor
help:
	@echo 'Tools: tools doctor; cluster: cluster-up cluster-configure cluster-down cluster-destroy CONFIRM=gpu-node-selector'
	@echo 'Images: docker-build minikube-load; Helm: helm-lint helm-test helm-install helm-aiworkloads helm-inference helm-status helm-verify'
	@echo 'Cleanup: helm-aiworkloads-remove helm-uninstall; tests: verify demo e2e-recovery lifecycle-check'
	@echo 'Development: generate manifests fmt vet test envtest build race check-generated; legacy deployment: install deploy'
tools:
	/bin/bash scripts/install-tools.sh
doctor:
	/bin/bash scripts/doctor.sh

.PHONY: cluster-up cluster-configure cluster-down cluster-destroy scheduling-proof scheduling-proof-cleanup
cluster-up cluster-configure cluster-down cluster-destroy:
	/bin/bash scripts/$@.sh
scheduling-proof:
	/bin/bash hack/scheduling-proof/run.sh
scheduling-proof-cleanup:
	/bin/bash hack/scheduling-proof/run.sh cleanup
export CONFIRM

GO ?= go
# The manager and API tests need no C dependencies. Keep builds portable to
# minimal container images and independent of the host's macOS SDK linker.
CGO_ENABLED ?= 0
export CGO_ENABLED
LOCALBIN := $(CURDIR)/bin
CONTROLLER_GEN := $(LOCALBIN)/controller-gen-v0.22.0
SETUP_ENVTEST := $(LOCALBIN)/setup-envtest-v0.25.0
ENVTEST_K8S_VERSION := 1.37.0

$(CONTROLLER_GEN):
	mkdir -p $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.22.0
	mv $(LOCALBIN)/controller-gen $@
$(SETUP_ENVTEST):
	mkdir -p $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install sigs.k8s.io/controller-runtime/tools/setup-envtest@v0.25.0
	mv $(LOCALBIN)/setup-envtest $@

.PHONY: generate manifests fmt vet test envtest build install api-smoke check-generated
generate: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) object paths="./api/..."
manifests: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) crd paths="./api/..." output:crd:artifacts:config=config/crd/bases
	$(CONTROLLER_GEN) rbac:roleName=aiworkload-manager-role paths="./internal/controller/..." output:rbac:artifacts:config=config/rbac
fmt:
	gofmt -w api cmd internal tests
vet:
	$(GO) vet ./...
test:
	$(GO) test ./...
envtest: manifests $(SETUP_ENVTEST)
	@assets="$$($(SETUP_ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN)/envtest -p path)" && KUBEBUILDER_ASSETS="$$assets" $(GO) test -tags=envtest -count=1 -v ./tests/envtest
build:
	mkdir -p $(LOCALBIN)
	$(GO) build -o $(LOCALBIN)/manager ./cmd
	$(GO) build -o $(LOCALBIN)/mock-inference ./cmd/mock-inference
	$(GO) build -o $(LOCALBIN)/fake-device-plugin ./cmd/fake-device-plugin
install: manifests
	/bin/bash scripts/install-crd.sh
api-smoke:
	/bin/bash scripts/api-smoke.sh
check-generated:
	/bin/bash scripts/check-generated.sh

.PHONY: mock-image mock-smoke race
mock-image:
	/bin/bash scripts/mock-image.sh
mock-smoke:
	/bin/bash scripts/mock-smoke.sh
race:
	# Match the pure-Go production resolver while retaining race instrumentation.
	CGO_ENABLED=1 $(GO) test -race -tags=netgo,osusergo ./internal/...

.PHONY: run
run: build
	/bin/bash scripts/run-manager.sh

.PHONY: docker-build minikube-load plugin-deploy plugin-verify plugin-restart
docker-build:
	/bin/bash scripts/images.sh build
minikube-load:
	/bin/bash scripts/images.sh load
plugin-deploy plugin-verify plugin-restart:
	/bin/bash scripts/plugin.sh $(@:plugin-%=%)

.PHONY: deploy demo-smoke
deploy: manifests
	/bin/bash scripts/deploy.sh
demo-smoke:
	/bin/bash scripts/demo.sh

.PHONY: e2e e2e-recovery
e2e:
	/bin/bash scripts/e2e.sh baseline
e2e-recovery:
	/bin/bash scripts/e2e.sh recovery

.PHONY: verify demo fmt-check shell-test lifecycle-check
lifecycle-check:
	/bin/bash scripts/lifecycle-proof.sh
verify:
	/bin/bash scripts/verify.sh
demo:
	E2E_EVIDENCE_ROOT="$(CURDIR)/work/day-10/demo" /bin/bash scripts/e2e.sh baseline
fmt-check:
	@test -z "$$(gofmt -l api cmd internal tests)" || { gofmt -l api cmd internal tests; echo 'FAIL: run make fmt'; exit 1; }
shell-test:
	@for file in scripts/*.sh scripts/lib/*.sh tests/e2e/*.sh; do bash -n "$$file" || exit; done
	@for file in scripts/test-*.sh; do bash "$$file" || exit; done

# Helm owns platform objects and AIWorkloads; the operator owns inference children.
.PHONY: helm-sync helm-lint helm-test helm-install helm-aiworkloads helm-inference helm-status helm-verify helm-aiworkloads-remove helm-uninstall
helm-sync: manifests
	cp config/crd/bases/ai.dhiraj.io_aiworkloads.yaml charts/gpu-node-selector/crds/ai.dhiraj.io_aiworkloads.yaml
helm-lint helm-install helm-aiworkloads helm-status helm-verify helm-aiworkloads-remove helm-uninstall:
	/bin/bash scripts/helm.sh $(@:helm-%=%)
helm-inference: helm-aiworkloads
helm-test: helm-lint
	$(GO) test -count=1 ./tests/helm
export HELM_VALUES
