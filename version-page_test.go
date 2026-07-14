package main

import "testing"

func TestSupportTier(t *testing.T) {
	tests := []struct {
		name                      string
		index                     int
		tag                       string
		stableReleasesSeen        int
		wantStatus                string
		wantClass                 string
		wantNewStableReleasesSeen int
	}{
		{name: "master", index: 0, tag: "master", stableReleasesSeen: 0, wantStatus: "Unsupported", wantClass: "diff-del", wantNewStableReleasesSeen: 0},
		{name: "rc release", index: 1, tag: "v1.8.0-rc1", stableReleasesSeen: 0, wantStatus: "Unsupported", wantClass: "diff-del", wantNewStableReleasesSeen: 0},
		{name: "first stable release", index: 2, tag: "v1.7.2", stableReleasesSeen: 0, wantStatus: "Supported", wantClass: "diff-add", wantNewStableReleasesSeen: 1},
		{name: "second stable release", index: 3, tag: "v1.7.1", stableReleasesSeen: 1, wantStatus: "Best-effort", wantClass: "text-muted", wantNewStableReleasesSeen: 2},
		{name: "third stable release", index: 4, tag: "v1.7.0", stableReleasesSeen: 2, wantStatus: "Unsupported", wantClass: "diff-del", wantNewStableReleasesSeen: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, class, newStableReleasesSeen := supportTier(tt.index, tt.tag, tt.stableReleasesSeen)
			if status != tt.wantStatus || class != tt.wantClass || newStableReleasesSeen != tt.wantNewStableReleasesSeen {
				t.Errorf("supportTier(%d, %q, %d) = (%q, %q, %d), want (%q, %q, %d)",
					tt.index, tt.tag, tt.stableReleasesSeen,
					status, class, newStableReleasesSeen,
					tt.wantStatus, tt.wantClass, tt.wantNewStableReleasesSeen)
			}
		})
	}
}
