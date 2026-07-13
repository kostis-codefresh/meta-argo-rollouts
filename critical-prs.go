package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v66/github"
)

// criticalPRRow is a ready PR that also deletes lines from an existing
// *_test.go file — changing test behavior for existing users, as opposed to
// only adding new tests or new assertions.
type criticalPRRow struct {
	Number    int
	Title     string
	HTMLURL   string
	Author    string
	CreatedAt time.Time
	Additions int
	Deletions int
}

// collectCriticalPRRows narrows the already-filtered ready PRs down to those
// that delete code in a unit or integration test file.
func collectCriticalPRRows(ctx context.Context, client *github.Client, readyRows []readyPRRow) []criticalPRRow {
	var rows []criticalPRRow
	for _, r := range readyRows {
		if !removesTestCode(ctx, client, r.Number) {
			continue
		}
		rows = append(rows, criticalPRRow{
			Number:    r.Number,
			Title:     r.Title,
			HTMLURL:   r.HTMLURL,
			Author:    r.Author,
			CreatedAt: r.CreatedAt,
			Additions: r.Additions,
			Deletions: r.Deletions,
		})
	}
	return rows
}

// removesTestCode reports whether the PR deletes any lines from a *_test.go
// file. Go unit tests live next to the code they test throughout the repo,
// and integration/e2e tests under test/e2e/ follow the same naming
// convention, so filename alone identifies both.
func removesTestCode(ctx context.Context, client *github.Client, number int) bool {
	opts := &github.ListOptions{PerPage: perPageMax}
	for {
		files, resp, err := client.PullRequests.ListFiles(ctx, "argoproj", "argo-rollouts", number, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error fetching files for PR #%d: %v\n", number, err)
			return false
		}
		for _, f := range files {
			if strings.HasSuffix(f.GetFilename(), "_test.go") && f.GetDeletions() > 0 {
				return true
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return false
}

// printCriticalPRRows prints one line per row, mirroring printReadyPRRows.
func printCriticalPRRows(rows []criticalPRRow) {
	for _, row := range rows {
		fmt.Printf("#%d - %s (by %s, opened %s) +%d/-%d\n", row.Number, row.Title, row.Author, row.CreatedAt.Format("2006-01-02"), row.Additions, row.Deletions)
	}
}
