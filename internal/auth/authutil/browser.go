package authutil

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// WaitForBrowserCallback lauches a new HTTP server listening on the provided
// address and waits for a request. Once received, the code is extracted from
// the query string (if any), and returned it to the caller.
func WaitForBrowserCallback(addr string) (code string, state string, err error) {
	type callback struct {
		code           string
		state          string
		err            string
		errDescription string
	}

	cbCh := make(chan *callback)
	errCh := make(chan error)

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
			_, _ = w.Write([]byte("<p>&#10060; Unable to extract code from request, please try authenticating again.</p>"))
		} else {
			_, _ = w.Write([]byte("<p>&#128075; You can close the window and go back to the CLI to see the user info and tokens.</p>"))
		}

		cbCh <- cb
	})

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case cb := <-cbCh:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		defer func(c context.Context) { _ = s.Shutdown(ctx) }(ctx)

		var err error
		if cb.err != "" {
			err = fmt.Errorf("%s: %s", cb.err, cb.errDescription)
		}
		return cb.code, cb.state, err
	case err := <-errCh:
		return "", "", err
	}
}
