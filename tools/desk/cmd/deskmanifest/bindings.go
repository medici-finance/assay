package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Declaration bindings select among installed providers for this linter only.
// They do not configure launchers or replace runtime roster-key activation.
func loadBindings(root string, providers map[string][]provider, exclusive map[string]bool) (map[string]string, error) {
	path := filepath.Join(root, "components", "bindings.yaml")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("bindings must be a regular file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Schema   int               `yaml:"schema"`
		Bindings map[string]string `yaml:"bindings"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err = decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("invalid declaration bindings: %w", err)
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, fmt.Errorf("trailing declaration bindings document")
	}
	if doc.Schema != 1 || len(doc.Bindings) == 0 {
		return nil, fmt.Errorf("declaration bindings require schema 1 and a nonempty map")
	}
	for key, id := range doc.Bindings {
		if !exclusive[key] {
			return nil, fmt.Errorf("binding key %s is not declared exclusive", key)
		}
		count := 0
		for _, p := range providers[key] {
			if p.component == id {
				count++
			}
		}
		if count != 1 {
			return nil, fmt.Errorf("binding %s selects unknown or ambiguous provider %s", key, id)
		}
	}
	return doc.Bindings, nil
}
