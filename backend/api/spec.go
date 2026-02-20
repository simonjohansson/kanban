package api

import _ "embed"

//go:embed openapi.yaml
var openAPIYAML []byte

func OpenAPIYAML() []byte {
	clone := make([]byte, len(openAPIYAML))
	copy(clone, openAPIYAML)
	return clone
}
