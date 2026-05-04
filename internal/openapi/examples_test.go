package openapi_test

import (
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"marbl/internal/httpapi"
	"marbl/internal/openapi"
)

type openAPISpec struct {
	Paths map[string]struct {
		Post struct {
			RequestBody struct {
				Content map[string]struct {
					Examples map[string]struct {
						Value map[string]int `yaml:"value"`
					} `yaml:"examples"`
				} `yaml:"content"`
			} `yaml:"requestBody"`
		} `yaml:"post"`
	} `yaml:"paths"`
}

func TestOpenAPIExampleDecodes(t *testing.T) {
	var doc openAPISpec
	if err := yaml.Unmarshal(openapi.Spec(), &doc); err != nil {
		t.Fatal(err)
	}
	ex := doc.Paths["/tasks"].Post.RequestBody.Content["application/json"].Examples["valid"].Value
	body := fmt.Sprintf(`{"id":%d,"type":%d,"value":%d}`, ex["id"], ex["type"], ex["value"])
	got, err := httpapi.DecodeTaskRequest(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != int64(ex["id"]) || got.Type != ex["type"] || got.Value != ex["value"] {
		t.Fatalf("decode mismatch: got %+v spec %v", got, ex)
	}
}
