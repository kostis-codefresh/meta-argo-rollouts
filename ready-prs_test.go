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
// .../requested_reviewers, for the given PR number. teamMembersJSON maps a
// team slug to the JSON array served for GET .../teams/{slug}/members; it
// may be nil if the test doesn't exercise team-based re-requests.
func newTestClient(t *testing.T, number int, reviewsJSON, reviewersJSON string, teamMembersJSON map[string]string) *github.Client {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/repos/argoproj/argo-rollouts/pulls/%d/reviews", number), func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, reviewsJSON)
	})
	mux.HandleFunc(fmt.Sprintf("/repos/argoproj/argo-rollouts/pulls/%d/requested_reviewers", number), func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, reviewersJSON)
	})
	for slug, membersJSON := range teamMembersJSON {
		mux.HandleFunc(fmt.Sprintf("/orgs/argoproj/teams/%s/members", slug), func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, membersJSON)
		})
	}

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
		name            string
		number          int
		reviewsJSON     string
		reviewersJSON   string
		teamMembersJSON map[string]string
		want            bool
	}{
		{
			// Two reviewers requested changes, but the only reviewer currently
			// requested is a third, different person. The PR must stay hidden
			// until one of the two who asked for changes is re-requested.
			name:   "changes requested, different reviewer re-requested",
			number: 4,
			reviewsJSON: `[
				{"user": {"login": "alice"}, "state": "CHANGES_REQUESTED", "author_association": "COLLABORATOR"},
				{"user": {"login": "bob"}, "state": "CHANGES_REQUESTED", "author_association": "COLLABORATOR"}
			]`,
			reviewersJSON: `{"users": [{"login": "carol"}], "teams": []}`,
			want:          false,
		},
		{
			name:   "changes requested, same reviewer re-requested",
			number: 1,
			reviewsJSON: `[
				{"user": {"login": "alice"}, "state": "CHANGES_REQUESTED", "author_association": "COLLABORATOR"}
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
				{"user": {"login": "alice"}, "state": "CHANGES_REQUESTED", "author_association": "COLLABORATOR"},
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
				{"user": {"login": "alice"}, "state": "CHANGES_REQUESTED", "author_association": "COLLABORATOR"}
			]`,
			reviewersJSON: `{"users": [], "teams": []}`,
			want:          false,
		},
		{
			// An outside contributor's changes-requested review carries no merge
			// authority either, so it must not block the PR.
			name:   "changes requested by an outside contributor does not hide it",
			number: 7,
			reviewsJSON: `[
				{"user": {"login": "dave"}, "state": "CHANGES_REQUESTED", "author_association": "CONTRIBUTOR"}
			]`,
			reviewersJSON: `{"users": [], "teams": []}`,
			want:          true,
		},
		{
			// The reviewer who requested changes was re-requested via a team
			// they belong to, rather than by name.
			name:   "changes requested, reviewer re-requested via a team",
			number: 8,
			reviewsJSON: `[
				{"user": {"login": "alice"}, "state": "CHANGES_REQUESTED", "author_association": "COLLABORATOR"}
			]`,
			reviewersJSON:   `{"users": [], "teams": [{"slug": "approvers"}]}`,
			teamMembersJSON: map[string]string{"approvers": `[{"login": "alice"}]`},
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, tt.number, tt.reviewsJSON, tt.reviewersJSON, tt.teamMembersJSON)
			got := needsReview(context.Background(), client, tt.number, map[string][]string{})
			if got != tt.want {
				t.Errorf("needsReview() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNeedsReviewCachesTeamMembers checks that resolving a requested team's
// members is done at most once per team per run, not once per PR, since the
// same handful of teams (e.g. argo-rollouts-approvers) get requested across
// many PRs.
func TestNeedsReviewCachesTeamMembers(t *testing.T) {
	teamMembersRequests := 0

	mux := http.NewServeMux()
	for _, number := range []int{10, 11} {
		number := number
		mux.HandleFunc(fmt.Sprintf("/repos/argoproj/argo-rollouts/pulls/%d/reviews", number), func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, `[{"user": {"login": "alice"}, "state": "CHANGES_REQUESTED", "author_association": "COLLABORATOR"}]`)
		})
		mux.HandleFunc(fmt.Sprintf("/repos/argoproj/argo-rollouts/pulls/%d/requested_reviewers", number), func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, `{"users": [], "teams": [{"slug": "approvers"}]}`)
		})
	}
	mux.HandleFunc("/orgs/argoproj/teams/approvers/members", func(w http.ResponseWriter, r *http.Request) {
		teamMembersRequests++
		_, _ = fmt.Fprint(w, `[{"login": "alice"}]`)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := github.NewClient(nil)
	baseURL, err := url.Parse(server.URL + "/")
	if err != nil {
		t.Fatalf("parsing test server URL: %v", err)
	}
	client.BaseURL = baseURL

	cache := map[string][]string{}
	for _, number := range []int{10, 11} {
		if !needsReview(context.Background(), client, number, cache) {
			t.Errorf("needsReview(%d) = false, want true", number)
		}
	}

	if teamMembersRequests != 1 {
		t.Errorf("team members endpoint hit %d times across 2 PRs sharing the same team, want 1", teamMembersRequests)
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
