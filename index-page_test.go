package main

import (
	"testing"
	"time"
)

func TestLatestStableRelease(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		rows            []releaseRow
		wantTag         string
		wantPublishedAt time.Time
	}{
		{
			name: "skips master, takes first stable",
			rows: []releaseRow{
				{Tag: "master"},
				{Tag: "v1.8.0", PublishedAt: t1},
				{Tag: "v1.7.2", PublishedAt: t2},
			},
			wantTag:         "v1.8.0",
			wantPublishedAt: t1,
		},
		{
			name: "skips leading RC releases",
			rows: []releaseRow{
				{Tag: "master"},
				{Tag: "v1.8.0-rc1"},
				{Tag: "v1.7.2", PublishedAt: t2},
			},
			wantTag:         "v1.7.2",
			wantPublishedAt: t2,
		},
		{
			name: "every release after master is an RC",
			rows: []releaseRow{
				{Tag: "master"},
				{Tag: "v1.8.0-rc1"},
			},
			wantTag:         "",
			wantPublishedAt: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTag, gotPublishedAt := latestStableRelease(tt.rows)
			if gotTag != tt.wantTag || !gotPublishedAt.Equal(tt.wantPublishedAt) {
				t.Errorf("latestStableRelease() = (%q, %v), want (%q, %v)", gotTag, gotPublishedAt, tt.wantTag, tt.wantPublishedAt)
			}
		})
	}
}
