MODULE   = $(shell env GO111MODULE=on $(GO) list -m)
DATE    ?= $(shell date +%FT%T%z)
VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || \
			cat $(CURDIR)/.version 2> /dev/null || echo v0)
COMMIT=$(shell git rev-parse HEAD)
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
GOVERSION=$(shell go version | awk -F\go '{print $$3}' | awk '{print $$1}')
PKGS     = $(or $(PKG),$(shell env GO111MODULE=on $(GO) list ./...))
TESTPKGS = $(shell env GO111MODULE=on $(GO) list -f \
			'{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' \
			$(PKGS))
BUILDPATH ?= $(BIN)
SRC_FILES=find . -name "*.go" -type f -not -path "./vendor/*" -not -path "./.git/*" -not -path "./.cache/*" -print0 | xargs -0 
BIN      = $(CURDIR)/bin
TBIN		 = $(CURDIR)/test/bin
INTDIR	 = $(CURDIR)/test/int-test
API_SERVICES=./api/services
GO			 = go
TIMEOUT  = 15
V = 0
Q = $(if $(filter 1,$V),,@)
M = $(shell printf "\033[34;1m➜\033[0m")
MAKEFLAGS += -j2

export GO111MODULE=on
export CGO_ENABLED=0

.PHONY: all
all: server node bbctl

# Run

.PHONY: run
run: run-server run-node

.PHONY: run-server
run-server: ; $(info $(M) running server) @ ## Run a server on localhost
	$Q $(GO) run cmd/blipblop-server/main.go \
		--tls-key ./certs/server.key \
		--tls-certificate ./certs/server.crt \
		--tls-host 0.0.0.0 \
		--tcp-tls-host 0.0.0.0

.PHONY: run-node
run-node: ; $(info $(M) running node) @ ## Run a node on localhost
	$Q $(GO) run cmd/blipblop-node/main.go --tls-ca ./certs/ca.crt --port 5743

# Build

.PHONY: server
server: | $(BIN) ; $(info $(M) building server executable to $(BUILDPATH)/$(BINARY_NAME)) @ ## Build program binary
	$Q $(GO) build \
		-tags release \
		-ldflags '-X main.VERSION=${VERSION} -X main.DATE=${DATE} -X main.COMMIT=${COMMIT} -X main.BRANCH=${BRANCH} -X main.GOVERSION=${GOVERSION}' \
		-o $(BUILDPATH)/$${BINARY_NAME:=blipblop-server} cmd/blipblop-server/main.go

.PHONY: node
node: | $(BIN) ; $(info $(M) building node executable to $(BUILDPATH)/$(BINARY_NAME)) @ ## Build program binary
	$Q $(GO) build \
		-tags release \
		-ldflags '-X main.VERSION=${VERSION} -X main.DATE=${DATE} -X main.COMMIT=${COMMIT} -X main.BRANCH=${BRANCH} -X main.GOVERSION=${GOVERSION}' \
		-o $(BUILDPATH)/$${BINARY_NAME:=blipblop-node} cmd/blipblop-node/main.go

.PHONY: bbctl
bbctl: | $(BIN) ; $(info $(M) building bbctl executable to $(BUILDPATH)/$(BINARY_NAME)) @ ## Build program binary
	$Q $(GO) build \
		-tags release \
		-ldflags '-X main.VERSION=${VERSION} -X main.DATE=${DATE} -X main.COMMIT=${COMMIT} -X main.BRANCH=${BRANCH} -X main.GOVERSION=${GOVERSION}' \
		-o $(BUILDPATH)/$${BINARY_NAME:=bbctl} main.go

.PHONY: docker_build
docker_build: ; $(info $(M) building docker image) @ ## Build docker image
	docker build -t amimof/blipblop:${VERSION} .
	dcker tag amimof/blipblop:${VERSION} amimof/blipblop:latest

.PHONY: protos
protos: $(API_SERVICES)/* ; $(info $(M) generating protos) @ ## Generate protos
	buf generate

# Tools

$(BIN):
	@mkdir -p $(BIN)
$(TBIN):
	@mkdir -p $@
$(INTDIR):
	@mkdir -p $@
$(TBIN)/%: | $(TBIN) ; $(info $(M) building $(PACKAGE))
	$Q tmp=$$(mktemp -d); \
	   env GOBIN=$(TBIN) $(GO) install $(PACKAGE) \
		|| ret=$$?; \
	   #rm -rf $$tmp ; exit $$ret

GOCILINT = $(TBIN)/golangci-lint
$(TBIN)/golangci-lint: PACKAGE=github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.0

# Tests
.PHONY: lint
lint: | $(GOCILINT) ; $(info $(M) running golangci-lint) @ ## Runs static code analysis using golangci-lint
	$Q $(GOCILINT) run --timeout=5m

.PHONY: test
test: ; $(info $(M) running go test) @ ## Runs unit tests
	$Q $(GO) test -count=1 -v ${PKGS}

.PHONY: fmt
fmt: ; $(info $(M) running gofmt) @ ## Formats Go code
	$Q $(GO) fmt $(PKGS)

.PHONY: vet
vet: ; $(info $(M) running go vet) @ ## Examines Go source code and reports suspicious constructs, such as Printf calls whose arguments do not align with the format string
	$Q $(GO) vet $(PKGS)

.PHONY: race
race: ; $(info $(M) running go race) @ ## Runs tests with data race detection
	$Q CGO_ENABLED=1 $(GO) test -race -short $(PKGS)

.PHONY: benchmark
benchmark: ; $(info $(M) running go benchmark test) @ ## Benchmark tests to examine performance
	$Q $(GO) test -run=__absolutelynothing__ -bench=. $(PKGS)

.PHONY: coverage
coverage: ; $(info $(M) running go coverage) @ ## Runs tests and generates code coverage report at ./test/coverage.out
	$Q mkdir -p $(CURDIR)/test/
	$Q $(GO) test -coverprofile="$(CURDIR)/test/coverage.out" $(PKGS)

.PHONY: checkfmt
checkfmt: ; $(info $(M) running checkfmt) @ ## Checks if code is formatted with go fmt and errors out if not
	@test "$(shell $(SRC_FILES) gofmt -l)" = "" \
    || { echo "Code not formatted, please run 'make fmt'"; exit 2; }

# Helm 

.PHONY: helm_lint
helm_lint: ; $(info $(M) running helm lint) @ ## Verifies that the chart is well-formed
	helm lint charts/blipblop/

# Misc

.PHONY: help
help:
	@grep -hE '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m∙ %s:\033[0m %s\n", $$1, $$2}'

.PHONY: version
version:	## Print version information
	@echo App: $(VERSION)
	@echo Go: $(GOVERSION)
	@echo Commit: $(COMMIT)
	@echo Branch: $(BRANCH)

.PHONY: clean
clean: ; $(info $(M) cleaning)	@ ## Cleanup everything
	@rm -rfv $(BIN)
	@rm -rfv $(TBIN)
	@rm -rfv $(CURDIR)/test
