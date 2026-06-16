package yamlutil

import (
	"strings"
	"testing"
)

func TestDecodeStrictHintsEndpointLabelFields(t *testing.T) {
	type endpoint struct {
		Node string `yaml:"node"`
	}
	type link struct {
		From endpoint `yaml:"from"`
		To   endpoint `yaml:"to"`
	}
	type document struct {
		Links []link `yaml:"links"`
	}

	input := []byte(`links:
  - from: {node: core-a}
    to: {node: edge-a}
    label_rotation: 90
`)
	var got document
	err := DecodeStrict(input, &got)
	if err == nil {
		t.Fatal("expected strict decode error")
	}
	if !strings.Contains(err.Error(), "must be nested under a link's from: or to: block") {
		t.Fatalf("missing endpoint-field hint: %v", err)
	}
}
