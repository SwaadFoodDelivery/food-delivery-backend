package repository

import "testing"

func TestMinorUnitsPreserveFractionalLeadingZeros(t *testing.T) {
	for raw, want := range map[string]int64{"0.05": 5, "12.09": 1209, "12.50": 1250, "12.5": 1250, "12": 1200, "0.00": 0, "259.95": 25995} {
		if got := mustMinor(raw); got != want {
			t.Errorf("%s: got%d want%d", raw, got, want)
		}
	}
}
