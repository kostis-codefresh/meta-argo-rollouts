package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/google/go-github/v66/github"
)

var apiCalls int

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
		fmt.Printf("%s - %s (%s)\n", r.GetTagName(), r.GetName(), r.GetPublishedAt().Format("2006-01-02"))
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
