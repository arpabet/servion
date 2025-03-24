VERSION := $(shell git describe --tags --always --dirty)
BUILD := $(shell date +"%m-%d-%Y")

all: build

version:
	@echo $(VERSION) $(BUILD)

install:
	go install go.arpabet.com/go-bindata/go-bindata@v1.0.0

bindata:
	go-bindata -pkg main -o bindata.go -nocompress -nomemcopy -fs -prefix "resources/" resources/...

update:
	go get -u ./...

build: bindata
	go build -v -ldflags "-X main.Version=$(VERSION) -X main.Build=$(BUILD)"









