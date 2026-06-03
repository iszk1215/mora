EXE = bin/mora

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -ldflags "-X github.com/iszk1215/mora/version.Version=$(VERSION) -X github.com/iszk1215/mora/version.Commit=$(COMMIT) -X github.com/iszk1215/mora/version.Date=$(DATE)"

all: $(EXE) coverage.html check

SOURCES = $(shell find . -name '*.go')

bin/mora: $(SOURCES)
	go build ./...
	go build $(LDFLAGS) -o $@ ./cmd

check: $(SOURCES)
	golangci-lint run
	# staticcheck ./...

test: $(SOURCES)
	go test -v ./...

run: bin/mora
	go test ./...
	bin/mora web --debug

coverage.out: $(SOURCES)
	go test -coverprofile=$@ ./...

coverage.html: coverage.out
	go tool cover -html=$< -o $@

clean:
	rm -f coverage.out coverage.html ${EXE}

generate:
	go generate mockscm/mock.go
	go generate udm/mock.go
