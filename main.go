package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/google/go-github/v66/github"
	"gopkg.in/yaml.v3"
)

var apiCalls int

var workflowFiles = []string{
	".github/workflows/testing.yaml",
	".github/workflows/e2e.yaml",
}

type countingTransport struct {
	base  http.RoundTripper
	token string
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	apiCalls++
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	return t.base.RoundTrip(req)
}

func main() {
	token := os.Getenv("GH_TOKEN")

	httpClient := &http.Client{
		Transport: &countingTransport{base: http.DefaultTransport, token: token},
	}
	client := github.NewClient(httpClient)

	ctx := context.Background()

	opts := &github.ListOptions{PerPage: 100}
	var releases []*github.RepositoryRelease
	for {
		page, resp, err := client.Repositories.ListReleases(ctx, "argoproj", "argo-rollouts", opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error fetching releases: %v\n", err)
			os.Exit(1)
		}
		releases = append(releases, page...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	for _, r := range releases {
		versions := fetchK8sVersions(ctx, client, r.GetTagName())
		versionStr := "(no k8s data)"
		if len(versions) > 0 {
			versionStr = "[" + strings.Join(versions, ", ") + "]"
		}
		fmt.Printf("%s - %s (%s) %s\n", r.GetTagName(), r.GetName(), r.GetPublishedAt().Format("2006-01-02"), versionStr)
	}

	rateLimits, _, err := client.RateLimits(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching rate limits: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("---")
	fmt.Printf("GitHub API calls made: %d\n", apiCalls)
	fmt.Printf("GitHub API rate limit remaining: %d / %d\n", rateLimits.Core.Remaining, rateLimits.Core.Limit)
}

// fetchK8sVersions returns the Kubernetes versions covered by the e2e test matrix
// at the given ref, trying testing.yaml first and falling back to the older e2e.yaml.
func fetchK8sVersions(ctx context.Context, client *github.Client, ref string) []string {
	for _, path := range workflowFiles {
		content, _, resp, err := client.Repositories.GetContents(ctx, "argoproj", "argo-rollouts", path, &github.RepositoryContentGetOptions{Ref: ref})
		if err != nil {
			if resp == nil || resp.StatusCode != http.StatusNotFound {
				fmt.Fprintf(os.Stderr, "error fetching %s at %s: %v\n", path, ref, err)
			}
			continue
		}

		raw, err := content.GetContent()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error decoding %s at %s: %v\n", path, ref, err)
			continue
		}

		var doc yaml.Node
		if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
			fmt.Fprintf(os.Stderr, "error parsing %s at %s: %v\n", path, ref, err)
			continue
		}

		if versions := findKubernetesVersions(&doc); len(versions) > 0 {
			return versions
		}
	}
	return nil
}

// findKubernetesVersions recursively walks a YAML tree looking for a "matrix" mapping
// that contains a key whose name includes "kubernetes", and extracts its versions.
func findKubernetesVersions(node *yaml.Node) []string {
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			if key.Value == "matrix" && value.Kind == yaml.MappingNode {
				if versions := extractFromMatrix(value); len(versions) > 0 {
					return versions
				}
			}
			if versions := findKubernetesVersions(value); len(versions) > 0 {
				return versions
			}
		}
		return nil
	}
	for _, child := range node.Content {
		if versions := findKubernetesVersions(child); len(versions) > 0 {
			return versions
		}
	}
	return nil
}

// extractFromMatrix scans a matrix mapping for a kubernetes-related key and returns
// its versions, handling both a list of {version: ...} objects and a flat scalar list.
func extractFromMatrix(matrix *yaml.Node) []string {
	for i := 0; i < len(matrix.Content); i += 2 {
		key, value := matrix.Content[i], matrix.Content[i+1]
		if strings.Contains(strings.ToLower(key.Value), "kubernetes") && value.Kind == yaml.SequenceNode {
			var versions []string
			for _, item := range value.Content {
				versions = append(versions, extractVersion(item))
			}
			return versions
		}
	}
	return nil
}

// extractVersion returns the raw scalar version string from a matrix entry, whether
// it's a bare scalar (e.g. 1.24) or a mapping with a "version" field (e.g. '1.32').
func extractVersion(item *yaml.Node) string {
	if item.Kind == yaml.MappingNode {
		for i := 0; i < len(item.Content); i += 2 {
			if item.Content[i].Value == "version" {
				return item.Content[i+1].Value
			}
		}
		return ""
	}
	return item.Value
}
