EXE = bin/mora

all: $(EXE) coverage.html check

SOURCES = $(shell find . -name '*.go')

bin/mora: $(SOURCES)
	go build ./...
	go build -o $@ main.go

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
