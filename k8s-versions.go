package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/google/go-github/v66/github"
	"gopkg.in/yaml.v3"
)

var workflowFiles = []string{
	".github/workflows/testing.yaml",
	".github/workflows/e2e.yaml",
}

const cacheDir = "cache"
const cacheFile = cacheDir + "/k8s-versions.json"

// cachedRelease is the on-disk record for a single release, keyed by tag in the cache file.
type cachedRelease struct {
	Name        string   `json:"name"`
	PublishedAt string   `json:"published_at"`
	K8sVersions []string `json:"k8s_versions"`
}

// loadCache reads the k8s-versions cache from disk, returning an empty map on a cold
// start (missing or unparseable file) rather than treating it as fatal.
func loadCache() map[string]cachedRelease {
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return map[string]cachedRelease{}
	}
	var cache map[string]cachedRelease
	if err := json.Unmarshal(data, &cache); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing %s: %v\n", cacheFile, err)
		return map[string]cachedRelease{}
	}
	return cache
}

// saveCache writes the k8s-versions cache to disk, creating the cache directory if needed.
func saveCache(cache map[string]cachedRelease) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating %s: %v\n", cacheDir, err)
		return
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error encoding %s: %v\n", cacheFile, err)
		return
	}
	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", cacheFile, err)
	}
}

// printReleasesWithK8sVersions lists every argoproj/argo-rollouts release along with
// the Kubernetes versions covered by that release's e2e test matrix. Releases already
// present in the on-disk cache are printed from the cache instead of being re-fetched.
func printReleasesWithK8sVersions(ctx context.Context, client *github.Client) {
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

	cache := loadCache()

	for _, r := range releases {
		tag := r.GetTagName()
		entry, ok := cache[tag]
		if !ok {
			entry = cachedRelease{
				Name:        r.GetName(),
				PublishedAt: r.GetPublishedAt().Format("2006-01-02"),
				K8sVersions: fetchK8sVersions(ctx, client, tag),
			}
			cache[tag] = entry
		}

		versionStr := "(no k8s data)"
		if len(entry.K8sVersions) > 0 {
			versionStr = "[" + strings.Join(entry.K8sVersions, ", ") + "]"
		}
		fmt.Printf("%s - %s (%s) %s\n", tag, entry.Name, entry.PublishedAt, versionStr)
	}

	saveCache(cache)
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
