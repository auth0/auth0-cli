package instrumentation

import (
	_ "embed"
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/auth0/auth0-cli/internal/buildinfo"
)

// SentryDSN is the destination for crash reports. A Sentry DSN is a public,
// write-only key that is safe to ship inside client binaries, so we hardcode a
// default here. This ensures crash reporting works for builds that are not
// produced by our release pipeline (for example Homebrew Core, which builds
// from source and cannot inject build-time values). Release builds may still
// override this via ldflags.
var SentryDSN = "https://370df87d33df46cb90182dd80a50fdc4@o27592.ingest.sentry.io/5694458"

// ReportException is designed to be called once as the CLI exits. We're
// purposefully initializing a client all the time given this context.
func ReportException(err error) bool {
	if SentryDSN == "" {
		return false
	}

	// Skip crash reporting for local/development builds so that dev-time panics
	// and errors are not shipped to Sentry. Release pipelines (goreleaser and
	// Homebrew Core) stamp a real semantic version via ldflags, whereas a local
	// `make build`/`make install` stamps "dev" and a plain `go build` leaves it
	// empty.
	if buildinfo.Version == "" || buildinfo.Version == "dev" {
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
