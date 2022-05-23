all: coverage.html check mora

SOURCES = $(shell find . -name '*.go')

mora: $(SOURCES)
	go build ./cmd/mora

check: $(SOURCES)
	staticcheck

test: $(SOURCES)
	go test -v

run: mora
	./mora -debug

coverage.out: $(SOURCES)
	go test -coverprofile=$@

coverage.html: coverage.out
	go tool cover -html=$< -o $@

clean:
	rm -f coverage.out coverage.html mora
