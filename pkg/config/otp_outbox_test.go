package config

import "testing"

func TestOTPOutboxRequiresExplicitLocalMock(t *testing.T) {
	for _, tc := range []struct {
		env, provider string
		wantErr       bool
	}{
		{"development", "mock", false}, {"test", "mock", false}, {"production", "mock", true}, {"staging", "mock", true}, {"development", "dev", true},
	} {
		t.Run(tc.env+"/"+tc.provider, func(t *testing.T) {
			t.Setenv("APP_ENV", tc.env)
			t.Setenv("OTP_PROVIDER", tc.provider)
			t.Setenv("MOCK_OTP_OUTBOX_DIR", t.TempDir())
			_, err := Load()
			if (err != nil) != tc.wantErr {
				t.Fatalf("configuration error=%v expectedError=%v", err, tc.wantErr)
			}
		})
	}
}
