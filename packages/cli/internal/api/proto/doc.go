// Package proto holds the protobuf specs for the sandbox-local (envd) APIs.
//
// process/process.proto is the source of truth for the process.Process
// ConnectRPC service; the generated Go code lives in ../processgen.
// Regenerate with:
//
//	go generate ./...
package proto

//go:generate go run github.com/bufbuild/buf/cmd/buf@v1.75.0 generate --template buf.gen.yaml .
