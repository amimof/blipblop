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

# Run

.PHONY: run
run: run-server run-node

.PHONY: run-server
run-server: ; $(info $(M) running server) @ ## Run a server on localhost
	$Q $(GO) run cmd/blipblop-server/main.go \
		--tls-key ./certs/server-key.pem \
		--tls-certificate ./certs/server.pem \
		--tls-host 0.0.0.0

.PHONY: run-node
run-node: ; $(info $(M) running node) @ ## Run a node on localhost
	$Q $(GO) run cmd/blipblop-node/main.go --node-name devnode 

# Build

.PHONY: all
all: server node

SERVER_BIN ?= blipblop-server
.PHONY: server
server: | $(BIN) ; $(info $(M) building server executable to $(BUILDPATH)/$(SERVER_BIN)) @ ## Build program binary
	$Q $(GO) build \
		-tags release \
		-ldflags '-X main.VERSION=${VERSION} -X main.COMMIT=${COMMIT} -X main.BRANCH=${BRANCH} -X main.GOVERSION=${GOVERSION}' \
		-o $(BUILDPATH)/$(SERVER_BIN) cmd/blipblop-server/main.go

NODE_BIN ?= blipblop-node
.PHONY: node
node: | $(BIN) ; $(info $(M) building node executable to $(BUILDPATH)/$(NODE_BIN)) @ ## Build program binary
	$Q $(GO) build \
		-tags release \
		-ldflags '-X main.VERSION=${VERSION} -X main.COMMIT=${COMMIT} -X main.BRANCH=${BRANCH} -X main.GOVERSION=${GOVERSION}' \
		-o $(BUILDPATH)/$(NODE_BIN) cmd/blipblop-node/main.go

.PHONY: docker_build
docker_build: ; $(info $(M) building docker image) @ ## Build docker image
	docker build -t amimof/blipblop:${VERSION} .
	docker tag amimof/blipblop:${VERSION} amimof/blipblop:latest

.PHONY: protos
protos: $(API_SERVICES)/* ; $(info $(M) generating protos) @ ## Generate protos
	@for d in $^; do \
		protoc \
			--proto_path=./$$d/v1 \
			--go_out=./$$d/v1 \
			--go-grpc_out=./$$d/v1 \
			--grpc-gateway_out=./$$d/v1 \
			--go_opt=paths=source_relative \
			--go-grpc_opt=paths=source_relative \
			--grpc-gateway_opt=logtostderr=true \
			--grpc-gateway_opt=paths=source_relative \
			--grpc-gateway_opt=generate_unbound_methods=true \
			./$$d/v1/*.proto ; \
		echo ./$$d/v1/*.proto ; \
	done

# Tools

$(BIN):
	@mkdir -p $(BIN)
$(TBIN):
	@mkdir -p $@
$(INTDIR):
	@mkdir -p $@
$(TBIN)/%: | $(TBIN) ; $(info $(M) building $(PACKAGE))
	$Q tmp=$$(mktemp -d); \
	   env GO111MODULE=off GOPATH=$$tmp GOBIN=$(TBIN) $(GO) get $(PACKAGE) \
		|| ret=$$?; \
	   rm -rf $$tmp ; exit $$ret

GOLINT = $(TBIN)/golint
$(BIN)/golint: PACKAGE=golang.org/x/lint/golint

GOCYCLO = $(TBIN)/gocyclo
$(TBIN)/gocyclo: PACKAGE=github.com/fzipp/gocyclo/cmd/gocyclo

INEFFASSIGN = $(TBIN)/ineffassign
$(TBIN)/ineffassign: PACKAGE=github.com/gordonklaus/ineffassign

MISSPELL = $(TBIN)/misspell
$(TBIN)/misspell: PACKAGE=github.com/client9/misspell/cmd/misspell

GOLINT = $(TBIN)/golint
$(TBIN)/golint: PACKAGE=golang.org/x/lint/golint

GOCOV = $(TBIN)/gocov
$(TBIN)/gocov: PACKAGE=github.com/axw/gocov/...

# Tests

.PHONY: lint
lint: | $(GOLINT) ; $(info $(M) running golint) @ ## Runs the golint command
	$Q $(GOLINT) -set_exit_status $(PKGS)

.PHONY: gocyclo
gocyclo: | $(GOCYCLO) ; $(info $(M) running gocyclo) @ ## Calculates cyclomatic complexities of functions in Go source code
	$Q $(GOCYCLO) -over 25 .

.PHONY: ineffassign
ineffassign: | $(INEFFASSIGN) ; $(info $(M) running ineffassign) @ ## Detects ineffectual assignments in Go code
	$Q $(INEFFASSIGN) ./...

.PHONY: misspell
misspell: | $(MISSPELL) ; $(info $(M) running misspell) @ ## Finds commonly misspelled English words
	$Q $(MISSPELL) .

.PHONY: test
test: ; $(info $(M) running go test) @ ## Runs unit tests
	$Q $(GO) test -v ${PKGS}

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