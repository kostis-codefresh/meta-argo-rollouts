package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v66/github"
	"gopkg.in/yaml.v3"
)

var workflowFiles = []string{
	".github/workflows/testing.yaml",
	".github/workflows/e2e.yaml",
}

const k8sVersionsCacheFile = "k8s-versions.json"

// cachedRelease is the on-disk record for a single release, keyed by tag in the cache file.
type cachedRelease struct {
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	K8sVersions []string  `json:"k8s_versions"`
}

// loadCache reads the k8s-versions cache from disk, returning an empty map on a cold
// start (missing or unparseable file) rather than treating it as fatal.
func loadCache() map[string]cachedRelease {
	cache := map[string]cachedRelease{}
	loadFromCache(k8sVersionsCacheFile, &cache)
	return cache
}

// saveCache writes the k8s-versions cache to disk, creating the cache directory if needed.
func saveCache(cache map[string]cachedRelease) {
	saveToCache(k8sVersionsCacheFile, cache)
}

const releaseCount = 30

// releaseRow is one entry (the master branch or a release) with its k8s test matrix.
type releaseRow struct {
	Tag         string
	Name        string
	PublishedAt time.Time
	K8sVersions []string
}

// collectReleaseRows fetches the master branch (always fresh, never cached) followed by
// the last 30 argoproj/argo-rollouts releases (using the on-disk cache), returning one
// row per entry with master first.
func collectReleaseRows(ctx context.Context, client *github.Client) []releaseRow {
	rows := []releaseRow{{
		Tag:         "master",
		Name:        "HEAD",
		PublishedAt: fetchMasterCommitDate(ctx, client),
		K8sVersions: fetchK8sVersions(ctx, client, "master"),
	}}

	opts := &github.ListOptions{PerPage: releaseCount}
	releases, _, err := client.Repositories.ListReleases(ctx, "argoproj", "argo-rollouts", opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching releases: %v\n", err)
		os.Exit(1)
	}

	cache := loadCache()

	for _, r := range releases {
		tag := r.GetTagName()
		entry, ok := cache[tag]
		if !ok {
			entry = cachedRelease{
				Name:        r.GetName(),
				PublishedAt: r.GetPublishedAt().Time,
				K8sVersions: fetchK8sVersions(ctx, client, tag),
			}
			cache[tag] = entry
		}
		rows = append(rows, releaseRow{Tag: tag, Name: entry.Name, PublishedAt: entry.PublishedAt, K8sVersions: entry.K8sVersions})
	}

	saveCache(cache)

	return rows
}

// fetchMasterCommitDate returns the committer date of master's tip commit.
func fetchMasterCommitDate(ctx context.Context, client *github.Client) time.Time {
	commit, _, err := client.Repositories.GetCommit(ctx, "argoproj", "argo-rollouts", "master", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching master commit date: %v\n", err)
		return time.Time{}
	}
	return commit.GetCommit().GetCommitter().GetDate().Time
}

// printReleaseRows prints one line per row, matching the previous console output format.
func printReleaseRows(rows []releaseRow) {
	for _, row := range rows {
		versionStr := "(no k8s data)"
		if len(row.K8sVersions) > 0 {
			versionStr = "[" + strings.Join(row.K8sVersions, ", ") + "]"
		}
		fmt.Printf("%s - %s (%s) %s\n", row.Tag, row.Name, row.PublishedAt.Format("2006-01-02"), versionStr)
	}
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
