package instrumentation

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/buildinfo"
)

func TestReportException(t *testing.T) {
	tests := []struct {
		name      string
		sentryDSN string
		version   string
		want      bool
	}{
		{
			name:      "skips when Sentry DSN is empty",
			sentryDSN: "",
			version:   "1.32.0",
			want:      false,
		},
		{
			name:      "skips for a plain go build with no version",
			sentryDSN: "https://public@o0.ingest.sentry.io/0",
			version:   "",
			want:      false,
		},
		{
			name:      "skips for a local dev build",
			sentryDSN: "https://public@o0.ingest.sentry.io/0",
			version:   "dev",
			want:      false,
		},
		{
			name:      "reports for a real release build",
			sentryDSN: "https://public@o0.ingest.sentry.io/0",
			version:   "1.32.0",
			want:      true,
		},
	}

	originalDSN := SentryDSN
	originalVersion := buildinfo.Version
	t.Cleanup(func() {
		SentryDSN = originalDSN
		buildinfo.Version = originalVersion
	})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			SentryDSN = test.sentryDSN
			buildinfo.Version = test.version

			got := ReportException(errors.New("boom"))

			assert.Equal(t, test.want, got)
		})
	}
}
