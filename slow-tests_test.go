package main

import "testing"

func TestParseSlowTests(t *testing.T) {
	log := `some unrelated log line
--- PASS: TestCanarySuite/TestCanaryProgressDeadlineExceededWithPause (12.500s)
--- FAIL: TestCanarySuite/TestCanaryFlaky (3.250s)
--- PASS: TestNoSuitePrefix (0.010s)
--- FAIL: TestBadDuration (1.2.3s)
`

	rows := parseSlowTests(log)

	want := []slowTestRow{
		{Suite: "TestCanarySuite", Name: "TestCanaryProgressDeadlineExceededWithPause", DurationSeconds: 12.5, Status: "PASS"},
		{Suite: "TestCanarySuite", Name: "TestCanaryFlaky", DurationSeconds: 3.25, Status: "FAIL"},
		{Suite: "", Name: "TestNoSuitePrefix", DurationSeconds: 0.01, Status: "PASS"},
	}

	if len(rows) != len(want) {
		t.Fatalf("parseSlowTests() returned %d rows, want %d: %+v", len(rows), len(want), rows)
	}
	for i, w := range want {
		if rows[i] != w {
			t.Errorf("parseSlowTests()[%d] = %+v, want %+v", i, rows[i], w)
		}
	}
}
