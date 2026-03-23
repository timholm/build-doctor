BINARY := build-doctor
MODULE := github.com/timholm/build-doctor
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test clean lint run docker

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test -race -count=1 ./...

clean:
	rm -f $(BINARY)
	go clean -testcache

lint:
	golangci-lint run ./...

run: build
	./$(BINARY)

docker:
	docker build -t $(BINARY):$(VERSION) .

fmt:
	gofmt -w .
	goimports -w .
