package main

import "testing"

func TestIsDocFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{name: "markdown at root", filename: "README.md", want: true},
		{name: "markdown nested", filename: "some/path/CHANGELOG.md", want: true},
		{name: "under docs folder", filename: "docs/getting-started.txt", want: true},
		{name: "docs folder nested deeper", filename: "website/docs/foo/bar.mdx", want: true},
		{name: "go source", filename: "main.go", want: false},
		{name: "docsomething not a docs folder", filename: "docsomething/file.go", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDocFile(tt.filename); got != tt.want {
				t.Errorf("isDocFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestExcludeDocOnly(t *testing.T) {
	ready := []readyPRRow{{Number: 1}, {Number: 2}, {Number: 3}}
	docs := []docPRRow{{Number: 2}}

	got := excludeDocOnly(ready, docs)

	want := []int{1, 3}
	if len(got) != len(want) {
		t.Fatalf("excludeDocOnly() returned %d rows, want %d", len(got), len(want))
	}
	for i, n := range want {
		if got[i].Number != n {
			t.Errorf("excludeDocOnly()[%d].Number = %d, want %d", i, got[i].Number, n)
		}
	}
}
