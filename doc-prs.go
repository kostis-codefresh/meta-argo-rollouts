package main

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/google/go-github/v66/github"
)

// docPRRow is a ready PR that only touches documentation files (Markdown
// files anywhere, or any file under a docs/ folder), as opposed to code.
type docPRRow struct {
	Number    int
	Title     string
	HTMLURL   string
	Author    string
	CreatedAt time.Time
	Additions int
	Deletions int
}

// collectDocPRRows narrows the already-filtered ready PRs down to those that
// only touch documentation files.
func collectDocPRRows(ctx context.Context, client *github.Client, readyRows []readyPRRow) []docPRRow {
	var rows []docPRRow
	for _, r := range readyRows {
		if !isDocOnly(ctx, client, r.Number) {
			continue
		}
		rows = append(rows, docPRRow(r))
	}
	return rows
}

// isDocOnly reports whether every file changed by the PR is a doc file.
func isDocOnly(ctx context.Context, client *github.Client, number int) bool {
	opts := &github.ListOptions{PerPage: perPageMax}
	sawFile := false
	for {
		files, resp, err := client.PullRequests.ListFiles(ctx, "argoproj", "argo-rollouts", number, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error fetching files for PR #%d: %v\n", number, err)
			return false
		}
		for _, f := range files {
			sawFile = true
			if !isDocFile(f.GetFilename()) {
				return false
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return sawFile
}

// isDocFile reports whether filename is a Markdown file, or lives under a
// docs/ folder anywhere in the path.
func isDocFile(filename string) bool {
	if strings.HasSuffix(filename, ".md") {
		return true
	}
	return slices.Contains(slices.Collect(strings.SplitSeq(filename, "/")), "docs")
}

// excludeDocOnly removes any row whose PR number also appears among the doc
// rows, so a PR classified as Doc PRs isn't duplicated on the Ready page.
func excludeDocOnly(readyRows []readyPRRow, docRows []docPRRow) []readyPRRow {
	docOnly := make(map[int]bool, len(docRows))
	for _, d := range docRows {
		docOnly[d.Number] = true
	}
	var rows []readyPRRow
	for _, r := range readyRows {
		if docOnly[r.Number] {
			continue
		}
		rows = append(rows, r)
	}
	return rows
}

// printDocPRRows prints one line per row, mirroring printCriticalPRRows.
func printDocPRRows(rows []docPRRow) {
	for _, row := range rows {
		fmt.Printf("#%d - %s (by %s, opened %s) +%d/-%d\n", row.Number, row.Title, row.Author, row.CreatedAt.Format("2006-01-02"), row.Additions, row.Deletions)
	}
}
