// Package docs embeds the generated OpenAPI 3.0 specification for the BusinessSAAS API.
//
// The swagger.json file is the canonical source of truth for API documentation.
// Regenerate it at any time by running: make docs
//
// This package exists solely to export the embedded spec so main.go can serve it
// without bundling the file as a separate asset at runtime.
//
//go:generate swag init -g ../../cmd/server/main.go --output . --oas3 --parseDependency --parseInternal
package docs

import _ "embed"

// SwaggerJSON is the complete OpenAPI 3.0.3 specification for the BusinessSAAS API.
// Embedded at build time so the binary is self-contained — no docs/ directory needed at runtime.
//
//go:embed swagger.json
var SwaggerJSON []byte
