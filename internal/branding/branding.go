package branding

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"text/template"
	"time"

	"github.com/auth0/auth0-cli/internal/open"
	"github.com/fsnotify/fsnotify"
	"github.com/guiguan/caster"
)

type Client struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	LogoURL string `json:"logo_url,omitempty"`
}

type TemplateData struct {
	Filename        string
	Clients         []Client
	PrimaryColor    string
	BackgroundColor string
	LogoURL         string
	TenantName      string
	Body            string
}

func PreviewCustomTemplate(ctx context.Context, data TemplateData) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return
	}
	address := listener.Addr().String()

	requestTimeout := 10 * time.Minute
	server := &http.Server{
		Handler:      buildRoutes(ctx, requestTimeout, data),
		ReadTimeout:  requestTimeout + 1*time.Minute,
		WriteTimeout: requestTimeout + 1*time.Minute,
	}
	defer server.Close()

	go func() {
		if err = server.Serve(listener); err != http.ErrServerClosed {
			cancel()
		}
	}()

	err = open.URL(fmt.Sprintf("http://%s/data/storybook/?path=/story/universal-login--prompts", address))
	if err != nil {
		return
	}

	// Wait until the file is closed or input is cancelled
	<-ctx.Done()
}

func buildRoutes(ctx context.Context, requestTimeout time.Duration, data TemplateData) *http.ServeMux {
	router := http.NewServeMux()

	// Long polling waiting for file changes
	broadcaster := broadcastCustomTemplateChanges(ctx, data.Filename)
	router.HandleFunc("/dynamic/events", func(w http.ResponseWriter, r *http.Request) {
		changes, _ := broadcaster.Sub(r.Context(), 1)
		defer broadcaster.Unsub(changes)

		var err error
		select {
		case <-r.Context().Done():
			w.WriteHeader(http.StatusGone)
			_, err = w.Write([]byte("410 - Gone"))
		case <-time.After(requestTimeout):
			w.WriteHeader(http.StatusRequestTimeout)
			_, err = w.Write([]byte("408 - Request Timeout"))
		case <-changes:
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte("200 - OK"))
		}

		if err != nil {
			http.Error(w, err.Error(), 500)
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

func broadcastCustomTemplateChanges(ctx context.Context, filename string) *caster.Caster {
	publisher := caster.New(ctx)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					publisher.Pub(true)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Fatal(err)
			}
		}
	}()

	go func() {
		<-ctx.Done()
		watcher.Close()
		publisher.Close()
	}()

	err = watcher.Add(filename)
	if err != nil {
		log.Fatal(err)
	}

	return publisher
}
