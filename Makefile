EXE = bin/mora

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -ldflags "-X github.com/iszk1215/mora/version.Version=$(VERSION) -X github.com/iszk1215/mora/version.Commit=$(COMMIT) -X github.com/iszk1215/mora/version.Date=$(DATE)"

FRONTEND_OUT := server/static/public/index.html

GO_PKGS = ./cmd/... ./coverage/... ./core/... ./mockscm/... ./render/... ./server/... ./udm/... ./version/...

.PHONY: all frontend frontend-test frontend-coverage test test-all test-race lint frontend-lint lint-all run clean generate

all: frontend $(EXE) coverage.html lint

SOURCES = $(shell find . -name '*.go' -not -path './frontend/node_modules/*')

bin/mora: $(SOURCES) $(FRONTEND_OUT)
	go build $(GO_PKGS)
	go build $(LDFLAGS) -o $@ main.go

lint: $(SOURCES)
	golangci-lint run $(GO_PKGS)

frontend-lint:
	$(MAKE) -C frontend lint

lint-all: lint frontend-lint

frontend-test:
	$(MAKE) -C frontend test
	# staticcheck ./...

test: test-race
	go test -v $(GO_PKGS)

test-race:
	go test -race -run 'TestMoraSession' -count=1 -timeout 30s ./server/

test-all: frontend-test test

run: bin/mora
	go test $(GO_PKGS)
	bin/mora web --debug

coverage.out: $(SOURCES)
	go test -coverprofile=$@ $(GO_PKGS)

coverage.html: coverage.out
	go tool cover -html=$< -o $@

frontend:
	$(MAKE) -C frontend build

frontend-coverage:
	$(MAKE) -C frontend coverage

coverage-all: coverage.out frontend-coverage

clean:
	rm -f coverage.out coverage.html ${EXE}

generate:
	go generate mockscm/mock.go
	go generate udm/mock.go
