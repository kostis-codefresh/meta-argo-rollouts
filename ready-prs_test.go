package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-github/v66/github"
)

// newTestClient returns a github.Client pointed at a test server that serves
// reviewsJSON for GET .../reviews and reviewersJSON for GET
// .../requested_reviewers, for the given PR number.
func newTestClient(t *testing.T, number int, reviewsJSON, reviewersJSON string) *github.Client {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/repos/argoproj/argo-rollouts/pulls/%d/reviews", number), func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, reviewsJSON)
	})
	mux.HandleFunc(fmt.Sprintf("/repos/argoproj/argo-rollouts/pulls/%d/requested_reviewers", number), func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, reviewersJSON)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := github.NewClient(nil)
	baseURL, err := url.Parse(server.URL + "/")
	if err != nil {
		t.Fatalf("parsing test server URL: %v", err)
	}
	client.BaseURL = baseURL
	return client
}

func TestNeedsReview(t *testing.T) {
	tests := []struct {
		name          string
		number        int
		reviewsJSON   string
		reviewersJSON string
		want          bool
	}{
		{
			// Two reviewers requested changes, but the only reviewer currently
			// requested is a third, different person. The PR must stay hidden
			// until one of the two who asked for changes is re-requested.
			name:   "changes requested, different reviewer re-requested",
			number: 4,
			reviewsJSON: `[
				{"user": {"login": "alice"}, "state": "CHANGES_REQUESTED"},
				{"user": {"login": "bob"}, "state": "CHANGES_REQUESTED"}
			]`,
			reviewersJSON: `{"users": [{"login": "carol"}], "teams": []}`,
			want:          false,
		},
		{
			name:   "changes requested, same reviewer re-requested",
			number: 1,
			reviewsJSON: `[
				{"user": {"login": "alice"}, "state": "CHANGES_REQUESTED"}
			]`,
			reviewersJSON: `{"users": [{"login": "alice"}], "teams": []}`,
			want:          true,
		},
		{
			name:          "no reviews yet",
			number:        2,
			reviewsJSON:   `[]`,
			reviewersJSON: `{"users": [], "teams": []}`,
			want:          true,
		},
		{
			name:   "approved by a collaborator hides it regardless of an earlier changes-requested",
			number: 3,
			reviewsJSON: `[
				{"user": {"login": "alice"}, "state": "CHANGES_REQUESTED"},
				{"user": {"login": "bob"}, "state": "APPROVED", "author_association": "COLLABORATOR"}
			]`,
			reviewersJSON: `{"users": [{"login": "alice"}], "teams": []}`,
			want:          false,
		},
		{
			// An outside contributor's approval carries no merge authority and
			// must not hide the PR.
			name:   "approved by an outside contributor does not hide it",
			number: 6,
			reviewsJSON: `[
				{"user": {"login": "dave"}, "state": "APPROVED", "author_association": "CONTRIBUTOR"}
			]`,
			reviewersJSON: `{"users": [], "teams": []}`,
			want:          true,
		},
		{
			name:   "changes requested, no one currently requested",
			number: 5,
			reviewsJSON: `[
				{"user": {"login": "alice"}, "state": "CHANGES_REQUESTED"}
			]`,
			reviewersJSON: `{"users": [], "teams": []}`,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, tt.number, tt.reviewsJSON, tt.reviewersJSON)
			got := needsReview(context.Background(), client, tt.number)
			if got != tt.want {
				t.Errorf("needsReview() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllChecksPassed(t *testing.T) {
	tests := []struct {
		name   string
		checks *github.ListCheckRunsResults
		want   bool
	}{
		{
			name:   "no check runs",
			checks: &github.ListCheckRunsResults{CheckRuns: nil},
			want:   false,
		},
		{
			name: "all success",
			checks: &github.ListCheckRunsResults{CheckRuns: []*github.CheckRun{
				{Status: new("completed"), Conclusion: new("success")},
				{Status: new("completed"), Conclusion: new("skipped")},
			}},
			want: true,
		},
		{
			name: "one still pending",
			checks: &github.ListCheckRunsResults{CheckRuns: []*github.CheckRun{
				{Status: new("completed"), Conclusion: new("success")},
				{Status: new("in_progress")},
			}},
			want: false,
		},
		{
			name: "one failed",
			checks: &github.ListCheckRunsResults{CheckRuns: []*github.CheckRun{
				{Status: new("completed"), Conclusion: new("success")},
				{Status: new("completed"), Conclusion: new("failure")},
			}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := allChecksPassed(tt.checks); got != tt.want {
				t.Errorf("allChecksPassed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExcludeCritical(t *testing.T) {
	ready := []readyPRRow{{Number: 1}, {Number: 2}, {Number: 3}}
	critical := []criticalPRRow{{Number: 2}}

	got := excludeCritical(ready, critical)

	want := []int{1, 3}
	if len(got) != len(want) {
		t.Fatalf("excludeCritical() returned %d rows, want %d", len(got), len(want))
	}
	for i, n := range want {
		if got[i].Number != n {
			t.Errorf("excludeCritical()[%d].Number = %d, want %d", i, got[i].Number, n)
		}
	}
}
