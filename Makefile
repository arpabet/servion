VERSION := $(shell git describe --tags --always --dirty)

all: build

version:
	@echo $(VERSION)

vet:
	go vet ./...
	cd grpc && go vet ./...

test: vet
	go test -cover -race ./...
	cd grpc && go test -cover -race ./...

build: test
	go build -v
	cd grpc && go build -v ./...

update:
	go get -u ./...

list:
	go list -m -json all | jq -r 'select(.GoVersion != null) | "\(.GoVersion) \(.Path)"' | sort -V
