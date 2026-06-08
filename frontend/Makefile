COVERAGE_OUT = coverage/lcov.info
SOURCES      = $(shell find src -type f 2>/dev/null)

.PHONY: build test dev lint coverage-report

build:
	npm run build

test:
	npm run test

$(COVERAGE_OUT): $(SOURCES)
	npm run test:coverage

coverage-report: $(COVERAGE_OUT)

lint:
	npm run lint

dev:
	npm run dev -- --no-open
