.PHONY: debug bin gitops install clean fmt vet depencencies lint ui ui-lint ui-test ui-dev unit-tests proto proto-deps api-dev ui-dev fakes crd
VERSION=$(shell git describe --always --match "v*")
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

BUILD_TIME=$(shell date +'%Y-%m-%d_%T')
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
GIT_COMMIT=$(shell git log -n1 --pretty='%h')
CURRENT_DIR=$(shell pwd)
FORMAT_LIST=$(shell gofmt -l .)
FLUX_VERSION=$(shell $(CURRENT_DIR)/tools/bin/stoml $(CURRENT_DIR)/tools/dependencies.toml flux.version)
LDFLAGS = "-X github.com/weaveworks/weave-gitops/cmd/gitops/version.BuildTime=$(BUILD_TIME) -X github.com/weaveworks/weave-gitops/cmd/gitops/version.Branch=$(BRANCH) -X github.com/weaveworks/weave-gitops/cmd/gitops/version.GitCommit=$(GIT_COMMIT) -X github.com/weaveworks/weave-gitops/pkg/version.FluxVersion=$(FLUX_VERSION)"

KUBEBUILDER_ASSETS ?= "$(CURRENT_DIR)/tools/bin/envtest"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

ifeq ($(BINARY_NAME),)
BINARY_NAME := gitops
endif

.PHONY: bin

all: gitops ## Install dependencies and build Gitops binary

##@ Test
unit-tests: dependencies cmd/gitops/ui/run/dist/index.html ## Run unit tests
	# To avoid downloading dependencies every time use `SKIP_FETCH_TOOLS=1 unit-tests`
	KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS) CGO_ENABLED=0 go test -v -tags unittest ./...

fakes: ## Generate testing fakes
	go generate ./...

##@ Build
gitops: dependencies ui bin ## Install dependencies and build gitops binary

install: bin ## Install binaries to GOPATH
	cp bin/$(BINARY_NAME) ${GOPATH}/bin/

api-dev: ## Server and watch gitops-server, will reload automatically on change
	reflex -r '.go' -s -- sh -c 'go run -ldflags $(LDFLAGS) cmd/gitops-server/main.go'

debug: ## Compile binary with optimisations and inlining disabled
	go build -ldflags $(LDFLAGS) -o bin/$(BINARY_NAME) -gcflags='all=-N -l' cmd/gitops/*.go

bin: ## Build gitops binary
	go build -ldflags $(LDFLAGS) -o bin/$(BINARY_NAME) cmd/gitops/*.go

docker: ## Build wego-app docker image
	docker build -t ghcr.io/weaveworks/wego-app:latest .


# Clean up images and binaries
clean: ## Clean up images and binaries
	rm -f bin/gitops
	rm -rf pkg/flux/bin/
	rm -rf cmd/gitops/ui/run/dist
	rm -rf coverage
	rm -rf node_modules
	rm -f .deps
	rm -rf dist

fmt: ## Run go fmt against code
	go fmt ./...

vet: ## Run go vet against code
	go vet ./...

lint: ## Run linters against code
	golangci-lint run --out-format=github-actions --build-tags acceptance

.deps:
	$(CURRENT_DIR)/tools/download-deps.sh $(CURRENT_DIR)/tools/dependencies.toml
	@touch .deps

dependencies: .deps ## Install build dependencies

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"
crd: ## Generate CRDs
	@go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1
	controller-gen $(CRD_OPTIONS) paths="./..." output:crd:artifacts:config=manifests/crds

check-format: ## Check go format
	if [ ! -z "$(FORMAT_LIST)" ] ; then echo invalid format at: ${FORMAT_LIST} && exit 1; fi

proto-deps: ## Update protobuf dependencies
	buf beta mod update

proto: ## Generate protobuf files
	buf generate
#	This job is complaining about a missing plugin and error-ing out
#	oapi-codegen -config oapi-codegen.config.yaml api/applications/applications.swagger.json

##@ UI

node_modules: ## Install node modules
	npm ci
	npx npm-force-resolutions

ui-lint: ## Run linter against the UI
	npm run lint

ui-test: ## Run UI tests
	npm run test

ui-audit: ## Run audit against the UI
	npm audit --production

ui: node_modules cmd/gitops/ui/run/dist/main.js ## Build the UI

ui-lib: node_modules dist/index.js dist/index.d.ts ## Build UI libraries
# Remove font files from the npm module.
	@find dist -type f -iname \*.otf -delete
	@find dist -type f -iname \*.woff -delete

cmd/gitops/ui/run/dist:
	mkdir -p cmd/gitops/ui/run/dist

cmd/gitops/ui/run/dist/index.html: cmd/gitops/ui/run/dist
	touch cmd/gitops/ui/run/dist/index.html

cmd/gitops/ui/run/dist/main.js:
	npm run build

# Runs a test to raise errors if the integration between Gitops Core and EE is
# in danger of breaking due to package API changes.
# See the test/library dockerfile and test.sh script for more info.
lib-test: dependencies ## Run the library integration test
	docker build -t gitops-library-test -f test/library/libtest.dockerfile $(CURRENT_DIR)/test/library
	docker run -e GITHUB_TOKEN=$$GITHUB_TOKEN -i --rm \
		-v $(CURRENT_DIR):/go/src/github.com/weaveworks/weave-gitops \
		 gitops-library-test

dist/index.js: ui/index.ts
	npm run build:lib && cp package.json dist

dist/index.d.ts: ui/index.ts
	npm run typedefs

# Test coverage

# JS coverage info
coverage/lcov.info:
	npm run test -- --coverage

# Golang gocov data. Not compatible with coveralls at this point.
coverage.out: dependencies
	go get github.com/ory/go-acc
	go-acc --ignore fakes,acceptance,pkg/api,api -o coverage.out ./... -- -v --timeout=496s -tags test
	@go mod tidy

# Convert gocov to lcov for coveralls
coverage/golang.info: coverage.out
	@mkdir -p coverage
	@go get -u github.com/jandelgado/gcov2lcov
	gcov2lcov -infile=coverage.out -outfile=coverage/golang.info

# Concat the JS and Go coverage files for the coveralls report/
# Note: you need to install `lcov` to run this locally.
coverage/merged.lcov: coverage/lcov.info coverage/golang.info
	lcov --add-tracefile coverage/golang.info -a coverage/lcov.info -o merged.lcov

##@ Utilities

.PHONY: help
# Thanks to https://www.thapaliya.com/en/writings/well-documented-makefiles/
help:  ## Display this help.
ifeq ($(OS),Windows_NT)
				@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make <target>\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-40s %s\n", $$1, $$2 } /^##@/ { printf "\n%s\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
else
				@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-40s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
endif