EXE = bin/mora

all: coverage.html check $(EXE)

SOURCES = $(shell find . -name '*.go')

$(EXE): $(SOURCES)
	go build -o $@ ./cmd/mora

check: $(SOURCES)
	staticcheck ./...

test: $(SOURCES)
	go test -v ./...

run: $(EXE)
	$(EXE) -debug

coverage.out: $(SOURCES)
	go test -coverprofile=$@ ./...

coverage.html: coverage.out
	go tool cover -html=$< -o $@

clean:
	rm -f coverage.out coverage.html bin/mora
