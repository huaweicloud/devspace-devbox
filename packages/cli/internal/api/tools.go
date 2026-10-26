//go:build tools

// Package api pins the oapi-codegen tool version for reproducible
// regeneration of the DevBox Sandbox Manager API client.
// See doc.go for the `go generate` directive.
package api

import (
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
)
