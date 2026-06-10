VERSION := $(shell git describe --tags --always --dirty)

all: build

version:
	@echo $(VERSION)

vet:
	go vet ./...

test: vet
	go test -cover -race ./...

build: test
	go build -v

update:
	go get -u ./...

list:
	go list -m -json all | jq -r 'select(.GoVersion != null) | "\(.GoVersion) \(.Path)"' | sort -V
