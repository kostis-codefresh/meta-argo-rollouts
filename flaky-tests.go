package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/google/go-github/v66/github"
)

const flakyRunCount = 10

// flakyTestRow is one test that failed at least once across the last
// flakyRunCount sampled master runs.
type flakyTestRow struct {
	Suite       string
	Name        string
	FailCount   int
	SampledRuns int // runs successfully sampled (<= flakyRunCount)
	LastFailure time.Time
}

// masterRun is a lightweight (ID, CreatedAt) pair for a completed master run.
type masterRun struct {
	ID        int64
	CreatedAt time.Time
}

// collectFlakyTestRows samples the last flakyRunCount completed (success or
// failure) master runs of the "Testing" workflow, downloads each one's
// "latest" k8s-matrix e2e job log, and returns every test that failed at
// least once, with its flake rate and most recent failure date. Never
// cached - this is a live snapshot, not accumulated history. A run whose job
// or log can't be fetched is skipped rather than aborting the whole
// collection.
func collectFlakyTestRows(ctx context.Context, client *github.Client) []flakyTestRow {
	runs := findRecentMasterRuns(ctx, client, flakyRunCount)

	failures := map[string]*flakyTestRow{}
	sampledRuns := 0

	for _, run := range runs {
		jobID, ok := findLatestMatrixJobID(ctx, client, run.ID)
		if !ok {
			continue
		}

		logText, err := downloadJobLog(ctx, client, jobID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error downloading log for job %d: %v\n", jobID, err)
			continue
		}
		sampledRuns++

		seen := map[string]bool{}
		for _, t := range parseSlowTests(logText) {
			if t.Status != "FAIL" {
				continue
			}
			key := t.Suite + "/" + t.Name
			if seen[key] {
				continue // the same FAIL line is sometimes printed twice in one log
			}
			seen[key] = true

			row, ok := failures[key]
			if !ok {
				row = &flakyTestRow{Suite: t.Suite, Name: t.Name}
				failures[key] = row
			}
			row.FailCount++
			if run.CreatedAt.After(row.LastFailure) {
				row.LastFailure = run.CreatedAt
			}
		}
	}

	rows := make([]flakyTestRow, 0, len(failures))
	for _, row := range failures {
		row.SampledRuns = sampledRuns
		rows = append(rows, *row)
	}

	sort.Slice(rows, func(i, j int) bool { return flakyLess(rows[i], rows[j]) })

	return rows
}

// flakyLess reports whether a should sort before b: higher flake rate first,
// then higher raw fail count as a tiebreaker.
func flakyLess(a, b flakyTestRow) bool {
	rateA := float64(a.FailCount) / float64(a.SampledRuns)
	rateB := float64(b.FailCount) / float64(b.SampledRuns)
	if rateA != rateB {
		return rateA > rateB
	}
	return a.FailCount > b.FailCount
}

// findRecentMasterRuns pages through completed "Testing" workflow runs on
// master, keeping only those with a success or failure conclusion (skipping
// cancelled/other runs, which carry no pass/fail signal), until n are found
// or pages run out.
func findRecentMasterRuns(ctx context.Context, client *github.Client, n int) []masterRun {
	var runs []masterRun
	opts := &github.ListWorkflowRunsOptions{
		Branch:      "master",
		Status:      "completed",
		ListOptions: github.ListOptions{PerPage: perPageMax},
	}
	for len(runs) < n {
		result, resp, err := client.Actions.ListWorkflowRunsByFileName(ctx, owner, repo, testingWorkflowFile, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error listing %s runs: %v\n", testingWorkflowFile, err)
			break
		}
		for _, r := range result.WorkflowRuns {
			conclusion := r.GetConclusion()
			if conclusion != "success" && conclusion != "failure" {
				continue
			}
			runs = append(runs, masterRun{ID: r.GetID(), CreatedAt: r.GetCreatedAt().Time})
			if len(runs) == n {
				break
			}
		}
		if resp.NextPage == 0 || len(result.WorkflowRuns) == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return runs
}

// printFlakyTestRows prints one line per row, matching the console-output
// convention of the other collectors.
func printFlakyTestRows(rows []flakyTestRow) {
	for _, row := range rows {
		fmt.Printf("%s - %s (%d/%d failed, last %s)\n", row.Suite, row.Name, row.FailCount, row.SampledRuns, row.LastFailure.Format("2006-01-02"))
	}
}
