all: coverage.html check mora

SOURCES = $(shell find . -name '*.go')

mora: $(SOURCES)
	go build -o bin/mora ./cmd/mora

check: $(SOURCES)
	staticcheck ./...

test: $(SOURCES)
	go test -v ./...

run: mora
	bin/mora -debug

coverage.out: $(SOURCES)
	go test -coverprofile=$@ ./...

coverage.html: coverage.out
	go tool cover -html=$< -o $@

clean:
	rm -f coverage.out coverage.html bin/mora
