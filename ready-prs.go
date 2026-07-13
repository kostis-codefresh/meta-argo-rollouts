package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/go-github/v66/github"
)

const perPageMax = 100

// readyPRRow is one open PR with no merge conflicts, no existing reviews,
// and all checks still running.
type readyPRRow struct {
	Number    int
	Title     string
	HTMLURL   string
	Author    string
	CreatedAt time.Time
}

// collectReadyPRRows fetches all open PRs and keeps only those with no merge
// conflicts, no reviews, and all checks still running. Always live — no on-disk
// cache, since this state is transient (unlike immutable releases).
func collectReadyPRRows(ctx context.Context, client *github.Client) []readyPRRow {
	prs := listAllOpenPRs(ctx, client)

	var rows []readyPRRow
	for _, pr := range prs {
		if isReadyToMerge(ctx, client, pr) {
			rows = append(rows, readyPRRow{
				Number:    pr.GetNumber(),
				Title:     pr.GetTitle(),
				HTMLURL:   pr.GetHTMLURL(),
				Author:    pr.GetUser().GetLogin(),
				CreatedAt: pr.GetCreatedAt().Time,
			})
		}
	}
	return rows
}

// listAllOpenPRs pages through every open PR against argoproj/argo-rollouts.
func listAllOpenPRs(ctx context.Context, client *github.Client) []*github.PullRequest {
	var all []*github.PullRequest
	opts := &github.PullRequestListOptions{
		State:       "open",
		ListOptions: github.ListOptions{PerPage: perPageMax},
	}
	for {
		prs, resp, err := client.PullRequests.List(ctx, "argoproj", "argo-rollouts", opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error listing open PRs: %v\n", err)
			os.Exit(1)
		}
		all = append(all, prs...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all
}

// isReadyToMerge applies the three-part filter, making up to 3 calls per PR
// (Get, ListReviews, ListCheckRunsForRef). Any error is treated as "not ready"
// rather than fatal, so one flaky PR lookup doesn't abort the whole run.
func isReadyToMerge(ctx context.Context, client *github.Client, pr *github.PullRequest) bool {
	number := pr.GetNumber()

	full, _, err := client.PullRequests.Get(ctx, "argoproj", "argo-rollouts", number)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching PR #%d: %v\n", number, err)
		return false
	}
	if full.GetMergeableState() == "dirty" {
		return false
	}

	reviews, _, err := client.PullRequests.ListReviews(ctx, "argoproj", "argo-rollouts", number, &github.ListOptions{PerPage: perPageMax})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching reviews for PR #%d: %v\n", number, err)
		return false
	}
	if len(reviews) != 0 {
		return false
	}

	checks, _, err := client.Checks.ListCheckRunsForRef(ctx, "argoproj", "argo-rollouts", pr.GetHead().GetSHA(), &github.ListCheckRunsOptions{ListOptions: github.ListOptions{PerPage: perPageMax}})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching check runs for PR #%d: %v\n", number, err)
		return false
	}
	return allChecksRunning(checks)
}

// allChecksRunning reports whether check runs exist for the ref and none have
// completed yet.
func allChecksRunning(checks *github.ListCheckRunsResults) bool {
	if len(checks.CheckRuns) == 0 {
		return false
	}
	for _, run := range checks.CheckRuns {
		if run.GetStatus() == "completed" {
			return false
		}
	}
	return true
}

// printReadyPRRows prints one line per row, mirroring printReleaseRows.
func printReadyPRRows(rows []readyPRRow) {
	for _, row := range rows {
		fmt.Printf("#%d - %s (by %s, opened %s)\n", row.Number, row.Title, row.Author, row.CreatedAt.Format("2006-01-02"))
	}
}
