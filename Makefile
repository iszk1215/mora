EXE = bin/mora

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -ldflags "-X github.com/iszk1215/mora/version.Version=$(VERSION) -X github.com/iszk1215/mora/version.Commit=$(COMMIT) -X github.com/iszk1215/mora/version.Date=$(DATE)"

SOURCES = $(shell find . -name '*.go' -not -path './frontend/node_modules/*')

FRONTEND_OUT := server/static/public/index.html
FRONTEND_SRCS := $(shell find frontend/src -type f 2>/dev/null)

GO_PKGS = ./cmd/... ./config/... ./coverage/... ./core/... ./mockscm/... ./render/... ./server/... ./track/... ./udm/... ./version/...

.PHONY: all frontend-test frontend-coverage test test-all test-race lint frontend-lint lint-all run clean generate

# all: frontend $(EXE) coverage.html lint
all: build-all test-all lint-all

build-all: frontend-build build

test-all: frontend-test test

lint-all: frontend-lint lint

coverage-all: coverage frontend-coverage


# backend

build: $(EXE)

bin/mora: $(SOURCES) $(FRONTEND_OUT)
	go build $(GO_PKGS)
	go build $(LDFLAGS) -o $@ main.go

lint: $(SOURCES)
	golangci-lint run $(GO_PKGS)

test: test-race
	go test $(GO_PKGS)

test-race:
	go test -race -run 'TestMoraSession' -count=1 -timeout 30s ./server/

coverage: coverage.out

coverage.out: $(SOURCES)
	go test -coverprofile=$@ $(GO_PKGS)

coverage.html: coverage.out
	go tool cover -html=$< -o $@

generate:
	go generate mockscm/mock.go
	go generate udm/mock.go

# frontend

frontend-build: ${FRONTEND_OUT}

$(FRONTEND_OUT): $(FRONTEND_SRCS)
	$(MAKE) -C frontend build

frontend-test:
	$(MAKE) -C frontend test
	# staticcheck ./...
	
frontend-lint:
	$(MAKE) -C frontend lint

frontend-coverage:
	$(MAKE) -C frontend coverage-report

# others

run: bin/mora
	go test $(GO_PKGS)
	bin/mora web --debug

clean:
	rm -f coverage.out coverage.html ${EXE}
