package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-github/v66/github"
)

const testingWorkflowFile = "testing.yaml"
const slowTestMinDuration = 5.0 // seconds; only tests slower than this are worth surfacing

// slowTestRow is one individual e2e test case's duration from the latest completed
// master run of the "Testing" workflow's "latest" k8s-matrix job.
type slowTestRow struct {
	Suite           string // e.g. "TestCanarySuite" ("" if the log line had no "/")
	Name            string // e.g. "TestCanaryProgressDeadlineExceededWithPause"
	DurationSeconds float64
	Status          string // "PASS" or "FAIL"
}

var slowTestLineRe = regexp.MustCompile(`--- (PASS|FAIL): (\S+) \(([0-9.]+)s\)`)

// collectSlowTestRows finds the most recent completed, successful run of the "Testing"
// workflow on master, locates its "latest" k8s-matrix e2e job, downloads that job's raw
// log, and returns every individual test slower than slowTestMinDuration, sorted slowest
// first. Never cached - this is a snapshot of the latest run, not accumulated history.
// Any failure at any stage is logged to stderr and yields an empty slice rather than
// aborting the program, since main.go treats each page independently.
func collectSlowTestRows(ctx context.Context, client *github.Client) []slowTestRow {
	runID, ok := findLatestMasterRunID(ctx, client)
	if !ok {
		return nil
	}

	jobID, ok := findLatestMatrixJobID(ctx, client, runID)
	if !ok {
		return nil
	}

	logText, err := downloadJobLog(ctx, client, jobID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error downloading log for job %d: %v\n", jobID, err)
		return nil
	}

	var rows []slowTestRow
	for _, row := range parseSlowTests(logText) {
		if row.DurationSeconds > slowTestMinDuration {
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].DurationSeconds > rows[j].DurationSeconds })
	return rows
}

// findLatestMasterRunID returns the run ID of the most recent completed, successful
// "Testing" workflow run on the master branch, or (0, false) if none is found or the
// API call fails.
func findLatestMasterRunID(ctx context.Context, client *github.Client) (int64, bool) {
	opts := &github.ListWorkflowRunsOptions{
		Branch:      "master",
		Status:      "success",
		ListOptions: github.ListOptions{PerPage: 1},
	}
	runs, _, err := client.Actions.ListWorkflowRunsByFileName(ctx, "argoproj", "argo-rollouts", testingWorkflowFile, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error listing %s runs: %v\n", testingWorkflowFile, err)
		return 0, false
	}
	if len(runs.WorkflowRuns) == 0 {
		fmt.Fprintf(os.Stderr, "no completed successful master runs found for %s\n", testingWorkflowFile)
		return 0, false
	}
	return runs.WorkflowRuns[0].GetID(), true
}

// findLatestMatrixJobID pages through the run's jobs looking for the one whose name
// ends in ", true)" (the "latest: true" k8s matrix entry) and contains "end-to-end
// tests", returning (0, false) if not found.
func findLatestMatrixJobID(ctx context.Context, client *github.Client, runID int64) (int64, bool) {
	opts := &github.ListWorkflowJobsOptions{ListOptions: github.ListOptions{PerPage: perPageMax}}
	for {
		jobs, resp, err := client.Actions.ListWorkflowJobs(ctx, "argoproj", "argo-rollouts", runID, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error listing jobs for run %d: %v\n", runID, err)
			return 0, false
		}
		for _, job := range jobs.Jobs {
			name := job.GetName()
			if strings.Contains(name, "end-to-end tests") && strings.HasSuffix(name, ", true)") {
				return job.GetID(), true
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	fmt.Fprintf(os.Stderr, "no latest-k8s-matrix e2e job found for run %d\n", runID)
	return 0, false
}

// downloadJobLog resolves the job's log redirect URL via go-github, then performs a
// plain http.Get to fetch the full plaintext log body. A plain client is used rather
// than the authenticated one, since the redirect target is a pre-signed storage URL
// that shouldn't get an Authorization header attached.
func downloadJobLog(ctx context.Context, client *github.Client, jobID int64) (string, error) {
	logURL, _, err := client.Actions.GetWorkflowJobLogs(ctx, "argoproj", "argo-rollouts", jobID, 10)
	if err != nil {
		return "", fmt.Errorf("resolving log URL: %w", err)
	}

	resp, err := http.Get(logURL.String())
	if err != nil {
		return "", fmt.Errorf("fetching log: %w", err)
	}
	defer closeAndLog(resp.Body, logURL.String())

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching log: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading log body: %w", err)
	}
	return string(body), nil
}

// parseSlowTests scans log text for "--- (PASS|FAIL): (\S+) \(([0-9.]+)s\)" lines and
// returns one slowTestRow per match (unsorted, uncapped).
func parseSlowTests(logText string) []slowTestRow {
	var rows []slowTestRow
	for _, match := range slowTestLineRe.FindAllStringSubmatch(logText, -1) {
		status, fullName, durationStr := match[1], match[2], match[3]

		duration, err := strconv.ParseFloat(durationStr, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error parsing duration %q for %s: %v\n", durationStr, fullName, err)
			continue
		}

		suite, name, hasSuite := strings.Cut(fullName, "/")
		if !hasSuite {
			suite, name = "", fullName
		}

		rows = append(rows, slowTestRow{Suite: suite, Name: name, DurationSeconds: duration, Status: status})
	}
	return rows
}

// printSlowTestRows prints one line per row, matching the console-output convention of
// the other collectors.
func printSlowTestRows(rows []slowTestRow) {
	for _, row := range rows {
		fmt.Printf("%s - %s (%s, %.2fs)\n", row.Suite, row.Name, row.Status, row.DurationSeconds)
	}
}
