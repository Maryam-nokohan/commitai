APP     ?= commitai
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
PREFIX  ?= $(HOME)/.local
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build install uninstall run test vet fmt tidy clean check

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(APP) ./cmd/commitai

install: build
	mkdir -p $(PREFIX)/bin
	install -m 0755 $(APP) $(PREFIX)/bin/$(APP)
	@echo ""
	@echo "commitai installed to $(PREFIX)/bin/$(APP)"
	@echo "Make sure $(PREFIX)/bin is in your PATH."

uninstall:
	rm -f $(PREFIX)/bin/$(APP)

run:
	go run ./cmd/commitai

test:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w ./cmd ./internal

tidy:
	go mod tidy

check: fmt vet test

clean:
	rm -f $(APP)
	rm -rf dist