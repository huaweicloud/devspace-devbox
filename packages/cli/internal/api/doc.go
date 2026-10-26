// Package api contains the typed client for the DevBox Sandbox Manager API.
//
// The code in client.gen.go is generated from openapi.yaml by oapi-codegen.
// Do not edit it by hand; regenerate with:
//
//	go generate ./...
//
// The openapi.yaml file at the module root is the source of truth, and the
// openapi spec is wire-compatible with the E2B API-SERVER contract.
package api

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -generate types,client -package api -o client.gen.go ../../openapi.yaml
