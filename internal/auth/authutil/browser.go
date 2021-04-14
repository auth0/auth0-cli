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
			_, _ = w.Write([]byte(resultPage("Login Failed", 
			"Unable to extract code from request, please try authenticating again.", 
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

func resultPage(title string, message string, iconClass string) string {
	html := `
<!DOCTYPE html>
<html>
<head>
	<title>Auth0 CLI</title>
	<link rel="shortcut icon" href="https://cdn.auth0.com/website/new-homepage/dark-favicon.png" id="favicon"/>
	<meta charset="utf-8" />
	<meta http-equiv="X-UA-Compatible" content="IE=edge" />
	<meta name="viewport" content="width=device-width, initial-scale=1" />	
	<link rel="stylesheet" href="https://cdn.auth0.com/ulp/react-components/1.46.14/css/main.cdn.min.css" />
	<style id="custom-styles-container">
	body {
	  background: #F0F1F3;
	  font-family: ulp-font, -apple-system, BlinkMacSystemFont, Roboto, Helvetica, sans-serif;
	}
	.caf10cc70 {
	  background: #F0F1F3;
	}
	.c404c83d8.c302c4ba1 {
	  background: #D00E17;
	}
	.c404c83d8.ccbc147ad {
	  background: #0A8852;
	}
	.c9ade4631 {
	  background-color: #635DFF;
	  color: #ffffff;
	}
	.c9ade4631 a,
	.c9ade4631 a:visited {
	  color: #ffffff;
	}
	.c87085aeb {
	  background-color: #0A8852;
	}
	.c8a6debc4 {
	  background-color: #D00E17;
	}
	.input.c911ad6ee {
	  border-color: #D00E17;
	}
	.error-cloud {
	  background-color: #D00E17;
	}
	.error-fatal {
	  background-color: #D00E17;
	}
	.error-local {
	  background-color: #D00E17;
	}
	.c03b79ab4.c41a3276a {
	  background-color: #D00E17;
	  border-color: #D00E17;
	}
	.c03b79ab4.c41a3276a::before {
	  border-bottom-color: #D00E17;
	}
	.c03b79ab4.c41a3276a::after {
	  border-bottom-color: #D00E17;
	}
	#alert-trigger {
	  background-color: #D00E17;
	}
	</style>
	<style>
	/* By default, hide features for javascript-disabled browsing */
	/* We use !important to override any css with higher specificity */
	/* It is also overriden by the styles in <noscript> in the header file */
	.no-js { display: none !important; }
	</style>
	<noscript>
	<style>
		/* We use !important to override the default for js enabled */
		/* If the display should be other than block, it should be defined specifically here */
		.js-required { display: none !important; }
		.no-js { display: block !important; }
	</style>
	</noscript>
	<style>.__s16nu9 {display:none;}</style>
</head>
<body class="_widget-auto-layout">
	<main class="_widget c58fd7e0a">
		<section class="cb38f2048 _prompt-box-outer ced5b89f0 c471522a0">
			<div class="ce794cbce c66a01acc">
			<div class="c507241b0 c0fc42669 ca8df1f97" data-event-id="">
				<div class="c598bf061 cc89f09a3">
					<span class="c05afb577 %s"></span>
				</div>
				<section class="c780c82ff c15e7affd">
					<h1 class="cd9c1d686 c67d39740 c0c89e88e">%s</h1>
					<div class="c6db31a88 c1728f1a3 cb059f28c">%s</div>
				</section>
			</div>
			</div>
		</section>
	</main>
</body>
</html>`

	return fmt.Sprintf(html, iconClass, title, message)
}
