package branding

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"text/template"
	"time"

	"github.com/auth0/auth0-cli/internal/open"
	"github.com/fsnotify/fsnotify"
	"github.com/guiguan/caster"
)

// Client is a minimal representation of an auth0 Client as defined in the
// management API. This is used within the branding machinery to populate the
// tenant data.
type Client struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	LogoURL string `json:"logo_url,omitempty"`
}

// TemplateData contains all the variables we project onto our embedded go
// template. These variables largely resemble the same ones in the auth0
// branding template.
type TemplateData struct {
	Filename        string
	Clients         []Client
	PrimaryColor    string
	BackgroundColor string
	LogoURL         string
	TenantName      string
	Body            string
}

func PreviewCustomTemplate(ctx context.Context, data TemplateData) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()

	// Long polling waiting for file changes
	broadcaster, err := broadcastCustomTemplateChanges(ctx, data.Filename)
	if err != nil {
		return err
	}

	requestTimeout := 10 * time.Minute
	server := &http.Server{
		Handler:      buildRoutes(ctx, requestTimeout, data, broadcaster),
		ReadTimeout:  requestTimeout + 1*time.Minute,
		WriteTimeout: requestTimeout + 1*time.Minute,
	}
	defer server.Close()

	go func() {
		if err = server.Serve(listener); err != http.ErrServerClosed {
			cancel()
		}
	}()

	u := &url.URL{
		Scheme: "http",
		Host:   listener.Addr().String(),
		Path:   "/data/storybook/",
		RawQuery: (url.Values{
			"path": []string{"/story/universal-login--prompts"},
		}).Encode(),
	}

	if err := open.URL(u.String()); err != nil {
		return err
	}

	// Wait until the file is closed or input is cancelled
	<-ctx.Done()
	return nil
}

func buildRoutes(ctx context.Context, requestTimeout time.Duration, data TemplateData, broadcaster *caster.Caster) *http.ServeMux {
	router := http.NewServeMux()

	router.HandleFunc("/dynamic/events", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		changes, _ := broadcaster.Sub(ctx, 1)
		defer broadcaster.Unsub(changes)

		writeStatus := func(w http.ResponseWriter, code int) {
			msg := fmt.Sprintf("%d - %s", code, http.StatusText(http.StatusGone))
			http.Error(w, msg, code)
		}

		select {
		case <-ctx.Done():
			writeStatus(w, http.StatusGone)
		case <-time.After(requestTimeout):
			writeStatus(w, http.StatusRequestTimeout)
		case <-changes:
			writeStatus(w, http.StatusOK)
		}
	})

	// The template file
	router.HandleFunc("/dynamic/template", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, data.Filename)
	})

	jstmpl := template.Must(template.New("tenant-data.js").Funcs(template.FuncMap{
		"asJS": func(v interface{}) string {
			a, _ := json.Marshal(v)
			return string(a)
		},
	}).Parse(tenantDataAsset))

	router.HandleFunc("/dynamic/tenant-data", func(w http.ResponseWriter, r *http.Request) {
		err := jstmpl.Execute(w, data)
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
	})

	// Storybook assets
	router.Handle("/", http.FileServer(http.FS(templatePreviewAssets)))

	return router
}

func broadcastCustomTemplateChanges(ctx context.Context, filename string) (*caster.Caster, error) {
	publisher := caster.New(ctx)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case _, ok := <-watcher.Events:
				if !ok {
					return
				}
				publisher.Pub(true)

			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	go func() {
		<-ctx.Done()
		watcher.Close()
		publisher.Close()
	}()

	if err := watcher.Add(filepath.Dir(filename)); err != nil {
		return nil, err
	}

	return publisher, nil
}
