package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/go-github/v66/github"
)

const perPageMax = 100

// readyPRRow is one open PR with no merge conflicts, all checks passing, and
// no approval yet (a PR with changes requested is excluded too, unless the
// author has since re-requested review from the same reviewer who requested
// changes).
type readyPRRow struct {
	Number    int
	Title     string
	HTMLURL   string
	Author    string
	CreatedAt time.Time
	Additions int
	Deletions int
}

// collectReadyPRRows fetches all open PRs and keeps only those with no merge
// conflicts, all checks passing, and no approval yet (a PR with changes
// requested is excluded too, unless the author has since re-requested review
// from the same reviewer who requested changes). Always live — no on-disk
// cache, since this state is transient (unlike immutable releases).
func collectReadyPRRows(ctx context.Context, client *github.Client) []readyPRRow {
	start := time.Now()
	fmt.Println("Starting to collect ready PRs")

	prs := listAllOpenPRs(ctx, client)
	fmt.Printf("Found %d total PRs\n", len(prs))

	var rows []readyPRRow
	for _, pr := range prs {
		full, ready := checkReadyToMerge(ctx, client, pr)
		if !ready {
			continue
		}
		rows = append(rows, readyPRRow{
			Number:    full.GetNumber(),
			Title:     full.GetTitle(),
			HTMLURL:   full.GetHTMLURL(),
			Author:    full.GetUser().GetLogin(),
			CreatedAt: full.GetCreatedAt().Time,
			Additions: full.GetAdditions(),
			Deletions: full.GetDeletions(),
		})
	}
	fmt.Printf("Finished ready PR step after %s\n", time.Since(start))
	return rows
}

// excludeCritical removes any row whose PR number also appears among the
// critical rows, so a PR classified as Critical isn't duplicated on the
// Ready page.
func excludeCritical(readyRows []readyPRRow, criticalRows []criticalPRRow) []readyPRRow {
	critical := make(map[int]bool, len(criticalRows))
	for _, c := range criticalRows {
		critical[c.Number] = true
	}
	var rows []readyPRRow
	for _, r := range readyRows {
		if critical[r.Number] {
			continue
		}
		rows = append(rows, r)
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
		prs, resp, err := client.PullRequests.List(ctx, owner, repo, opts)
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

// checkReadyToMerge applies the filter, making up to 4 calls per PR (Get,
// ListReviews, ListReviewers, ListCheckRunsForRef). It returns the fetched PR
// (with fields like Additions/Deletions only present on the single-PR Get
// response) alongside whether it passed. Any error is treated as "not ready"
// rather than fatal, so one flaky PR lookup doesn't abort the whole run.
func checkReadyToMerge(ctx context.Context, client *github.Client, pr *github.PullRequest) (*github.PullRequest, bool) {
	number := pr.GetNumber()

	full, _, err := client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching PR #%d: %v\n", number, err)
		return nil, false
	}
	if full.GetMergeableState() == "dirty" {
		return nil, false
	}

	if !needsReview(ctx, client, number) {
		return nil, false
	}

	checks, _, err := client.Checks.ListCheckRunsForRef(ctx, owner, repo, pr.GetHead().GetSHA(), &github.ListCheckRunsOptions{ListOptions: github.ListOptions{PerPage: perPageMax}})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching check runs for PR #%d: %v\n", number, err)
		return nil, false
	}
	if !allChecksPassed(checks) {
		return nil, false
	}
	return full, true
}

// needsReview reports whether the PR still needs a human verdict: no
// reviews yet, only comments so far, or changes were requested and the
// same reviewer who requested them has since been re-requested. An approval
// from someone with write access to the repo hides the PR permanently,
// regardless of any other outstanding review requests; an approval from an
// outside contributor (no merge authority) does not count.
func needsReview(ctx context.Context, client *github.Client, number int) bool {
	reviews, _, err := client.PullRequests.ListReviews(ctx, owner, repo, number, &github.ListOptions{PerPage: perPageMax})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching reviews for PR #%d: %v\n", number, err)
		return false
	}
	if len(reviews) == 0 {
		return true
	}

	changesRequestedBy := map[string]bool{}
	for _, review := range reviews {
		switch review.GetState() {
		case "APPROVED":
			if hasWriteAccess(review) {
				return false
			}
		case "CHANGES_REQUESTED":
			changesRequestedBy[review.GetUser().GetLogin()] = true
		}
	}
	if len(changesRequestedBy) == 0 {
		return true
	}

	reviewers, _, err := client.PullRequests.ListReviewers(ctx, owner, repo, number, &github.ListOptions{PerPage: perPageMax})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching requested reviewers for PR #%d: %v\n", number, err)
		return false
	}
	for _, user := range reviewers.Users {
		if changesRequestedBy[user.GetLogin()] {
			return true
		}
	}
	return false
}

// hasWriteAccess reports whether the review's author has write access to the
// repo, based on GitHub's author_association field. An OWNER, MEMBER, or
// COLLABORATOR can actually merge the PR; a CONTRIBUTOR or other outside
// association is someone who merely has commit history or no relationship
// with the repo, so their approval carries no merge authority.
func hasWriteAccess(review *github.PullRequestReview) bool {
	switch review.GetAuthorAssociation() {
	case "OWNER", "MEMBER", "COLLABORATOR":
		return true
	default:
		return false
	}
}

// allChecksPassed reports whether check runs exist for the ref and every one
// has completed with a passing conclusion (success or skipped).
func allChecksPassed(checks *github.ListCheckRunsResults) bool {
	if len(checks.CheckRuns) == 0 {
		return false
	}
	for _, run := range checks.CheckRuns {
		if run.GetStatus() != "completed" {
			return false
		}
		switch run.GetConclusion() {
		case "success", "skipped":
		default:
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
