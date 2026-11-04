package authutil

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"time"
)

// browserCallbackTimeout bounds how long WaitForBrowserCallback blocks waiting
// for the browser redirect. Without it a caller that never completes the login
// (for example an agent that emitted the URL but whose human walked away) would
// hang forever. It is generous enough for a human to finish logging in.
const browserCallbackTimeout = 5 * time.Minute

// WaitForBrowserCallback launches a new HTTP server listening on the provided
// address and waits for a request. Once received, the code is extracted from
// the query string (if any), and returned to the caller. The wait is bounded:
// it aborts if ctx is cancelled or browserCallbackTimeout elapses first, so a
// non-interactive caller never hangs indefinitely.
func WaitForBrowserCallback(ctx context.Context, addr string) (code string, state string, err error) {
	type callback struct {
		code           string
		state          string
		err            string
		errDescription string
	}

	cbCh := make(chan *callback, 1)
	errCh := make(chan error, 1)

	m := http.NewServeMux()
	s := &http.Server{Addr: addr, Handler: m}

	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		cb := &callback{
			code:           r.URL.Query().Get("code"),
			state:          r.URL.Query().Get("state"),
			err:            r.URL.Query().Get("error"),
			errDescription: r.URL.Query().Get("error_description"),
		}

		if cb.code == "" {
			_, _ = w.Write([]byte(resultPage("Login Failed",
				"Failed to extract code from request, please try authenticating again.",
				"error-denied")))
		} else {
			_, _ = w.Write([]byte(resultPage("Login Successful",
				"You can close the window and go back to the CLI to see the user info and tokens.",
				"success-lock")))
		}

		cbCh <- cb
	})

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Shutdown gives the server a brief window to close cleanly regardless of
	// which branch below returns.
	shutdown := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(shutdownCtx)
	}

	timeout := time.NewTimer(browserCallbackTimeout)
	defer timeout.Stop()

	select {
	case cb := <-cbCh:
		defer shutdown()

		var err error
		if cb.err != "" {
			err = fmt.Errorf("%s: %s", cb.err, cb.errDescription)
		}
		return cb.code, cb.state, err
	case err := <-errCh:
		shutdown()
		return "", "", err
	case <-ctx.Done():
		shutdown()
		return "", "", ctx.Err()
	case <-timeout.C:
		shutdown()
		return "", "", fmt.Errorf("timed out after %s waiting for the browser login callback", browserCallbackTimeout)
	}
}

//go:embed data/result-page.html
var resultHTML string

func resultPage(title string, message string, iconClass string) string {
	return fmt.Sprintf(resultHTML, iconClass, title, message)
}
