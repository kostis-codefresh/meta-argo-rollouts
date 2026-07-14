package main

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func mustParseYAMLNode(t *testing.T, raw string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("parsing test YAML: %v", err)
	}
	return doc.Content[0]
}

func TestFindKubernetesVersions(t *testing.T) {
	tests := []struct {
		name         string
		yaml         string
		wantVersions []string
		wantLine     int
	}{
		{
			name: "flat scalar list nested under jobs/strategy/matrix",
			yaml: `
jobs:
  test:
    strategy:
      matrix:
        kubernetes-version: ['1.24', '1.25']
`,
			wantVersions: []string{"1.24", "1.25"},
			wantLine:     6,
		},
		{
			name: "list of version objects",
			yaml: `
matrix:
  kubernetes-version:
    - version: '1.24'
    - version: '1.25'
`,
			wantVersions: []string{"1.24", "1.25"},
			wantLine:     3,
		},
		{
			name:         "no matrix at all",
			yaml:         "jobs:\n  test:\n    runs-on: ubuntu-latest\n",
			wantVersions: nil,
			wantLine:     0,
		},
		{
			name:         "matrix without a kubernetes key",
			yaml:         "matrix:\n  os: [ubuntu-latest]\n",
			wantVersions: nil,
			wantLine:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.yaml), &doc); err != nil {
				t.Fatalf("parsing test YAML: %v", err)
			}
			gotVersions, gotLine := findKubernetesVersions(&doc)
			if !reflect.DeepEqual(gotVersions, tt.wantVersions) {
				t.Errorf("findKubernetesVersions() versions = %v, want %v", gotVersions, tt.wantVersions)
			}
			if gotLine != tt.wantLine {
				t.Errorf("findKubernetesVersions() line = %d, want %d", gotLine, tt.wantLine)
			}
		})
	}
}

func TestExtractFromMatrix(t *testing.T) {
	tests := []struct {
		name         string
		yaml         string
		wantVersions []string
	}{
		{
			name:         "flat scalar list",
			yaml:         "kubernetes-version: ['1.24', '1.25']\n",
			wantVersions: []string{"1.24", "1.25"},
		},
		{
			name:         "list of version objects",
			yaml:         "kubernetes-version:\n  - version: '1.24'\n  - version: '1.25'\n",
			wantVersions: []string{"1.24", "1.25"},
		},
		{
			name:         "no kubernetes key",
			yaml:         "os: [ubuntu-latest]\n",
			wantVersions: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matrix := mustParseYAMLNode(t, tt.yaml)
			gotVersions, _ := extractFromMatrix(matrix)
			if !reflect.DeepEqual(gotVersions, tt.wantVersions) {
				t.Errorf("extractFromMatrix() versions = %v, want %v", gotVersions, tt.wantVersions)
			}
		})
	}
}

func TestExtractVersion(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{name: "bare scalar", yaml: "1.24\n", want: "1.24"},
		{name: "mapping with version field", yaml: "version: '1.32'\n", want: "1.32"},
		{name: "mapping without version field", yaml: "os: ubuntu-latest\n", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := mustParseYAMLNode(t, tt.yaml)
			if got := extractVersion(item); got != tt.want {
				t.Errorf("extractVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}
