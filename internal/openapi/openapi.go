package openapi

import _ "embed"

//go:embed spec.yaml
var spec []byte

func Spec() []byte {
	return spec
}
