package instrumentation

import (
	_ "embed"
	"time"

	"github.com/getsentry/sentry-go"
)

var SentryDSN string

// ReportException is designed to be called once as the CLI exits. We're
// purposefully initializing a client all the time given this context.
func ReportException(err error) bool {
	if SentryDSN == "" {
		return false
	}

	if err := sentry.Init(sentry.ClientOptions{Dsn: SentryDSN}); err != nil {
		return false
	}

	// Flush buffered events before the program terminates.
	sentry.CaptureException(err)

	// Allow up to 2s to flush, otherwise quit.
	sentry.Flush(2 * time.Second)
	return true
}
