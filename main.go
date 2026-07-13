package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

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

	generatedAt := time.Now()
	rows := collectReleaseRows(ctx, client)
	printReleaseRows(rows)
	if err := renderVersionPage(rows, generatedAt); err != nil {
		fmt.Fprintf(os.Stderr, "error rendering version page: %v\n", err)
	}

	readyRows := collectReadyPRRows(ctx, client)
	printReadyPRRows(readyRows)
	if err := renderReadyPage(readyRows, generatedAt); err != nil {
		fmt.Fprintf(os.Stderr, "error rendering ready page: %v\n", err)
	}

	criticalRows := collectCriticalPRRows(ctx, client, readyRows)
	printCriticalPRRows(criticalRows)
	if err := renderCriticalPage(criticalRows, generatedAt); err != nil {
		fmt.Fprintf(os.Stderr, "error rendering critical page: %v\n", err)
	}

	rateLimits, _, err := client.RateLimit.Get(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching rate limits: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("---")
	fmt.Printf("GitHub API calls made: %d\n", apiCalls)
	fmt.Printf("GitHub API rate limit remaining: %d / %d\n", rateLimits.Core.Remaining, rateLimits.Core.Limit)
}
