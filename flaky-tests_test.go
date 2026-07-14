package main

import "testing"

func TestFlakyLess(t *testing.T) {
	tests := []struct {
		name string
		a, b flakyTestRow
		want bool
	}{
		{
			name: "higher rate sorts first",
			a:    flakyTestRow{FailCount: 5, SampledRuns: 10}, // 50%
			b:    flakyTestRow{FailCount: 2, SampledRuns: 10}, // 20%
			want: true,
		},
		{
			name: "lower rate sorts after",
			a:    flakyTestRow{FailCount: 2, SampledRuns: 10}, // 20%
			b:    flakyTestRow{FailCount: 5, SampledRuns: 10}, // 50%
			want: false,
		},
		{
			name: "equal rate, higher fail count sorts first",
			a:    flakyTestRow{FailCount: 4, SampledRuns: 8}, // 50%
			b:    flakyTestRow{FailCount: 2, SampledRuns: 4}, // 50%
			want: true,
		},
		{
			name: "equal rate and count",
			a:    flakyTestRow{FailCount: 2, SampledRuns: 4},
			b:    flakyTestRow{FailCount: 2, SampledRuns: 4},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flakyLess(tt.a, tt.b); got != tt.want {
				t.Errorf("flakyLess(%+v, %+v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
