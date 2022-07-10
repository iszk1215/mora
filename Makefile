EXE = bin/mora
EXE += bin/upload

all: coverage.html check $(EXE)

SOURCES = $(shell find . -name '*.go')

bin/mora: $(SOURCES)
	go build -o $@ ./cmd/mora

bin/upload: $(SOURCES)
	go build -o $@ ./cmd/upload

check: $(SOURCES)
	staticcheck ./...

test: $(SOURCES)
	go test -v ./...

run: bin/mora
	go test ./...
	bin/mora -debug

coverage.out: $(SOURCES)
	go test -coverprofile=$@ ./...

coverage.html: coverage.out
	go tool cover -html=$< -o $@

clean:
	rm -f coverage.out coverage.html ${EXE}
