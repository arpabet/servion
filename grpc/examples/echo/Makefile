# Regenerate the Go gRPC stubs from echopb/echo.proto.
#
# Requires protoc plus the Go plugins on PATH:
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

generate:
	protoc --go_out=. --go_opt=paths=source_relative \
	       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	       echopb/echo.proto

.PHONY: generate
