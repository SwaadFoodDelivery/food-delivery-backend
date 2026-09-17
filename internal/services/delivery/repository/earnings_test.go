package repository

import "testing"

func TestDecimalToMinor(t *testing.T) {
	cases := []struct {
		raw     string
		want    int64
		wantErr bool
	}{
		{raw: "30.00", want: 3000},
		{raw: "30", want: 3000},
		{raw: "0.05", want: 5},
		{raw: "0", want: 0},
		{raw: "136.05", want: 13605},
		{raw: "-1.00", wantErr: true},
		{raw: "1.005", wantErr: true},
		{raw: "not-a-number", wantErr: true},
		{raw: "", wantErr: true},
	}
	for _, tc := range cases {
		got, err := decimalToMinor(tc.raw)
		if tc.wantErr {
			if err == nil {
				t.Errorf("decimalToMinor(%q) = %d, want an error", tc.raw, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("decimalToMinor(%q) unexpected error: %v", tc.raw, err)
			continue
		}
		if got != tc.want {
			t.Errorf("decimalToMinor(%q) = %d, want %d", tc.raw, got, tc.want)
		}
	}
}

func TestMustMinorIgnoresParseFailure(t *testing.T) {
	if got := mustMinor("garbage"); got != 0 {
		t.Fatalf("mustMinor(garbage) = %d, want 0", got)
	}
}
