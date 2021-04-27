package branding

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"text/template"
	"time"

	"github.com/auth0/auth0-cli/internal/open"
	"github.com/guiguan/caster"
	"github.com/phayes/freeport"
	"github.com/rjeczalik/notify"
)

type Client struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	LogoUrl string `json:"logo_url,omitempty"`
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

func PreviewCustomTemplate(ctx context.Context, templateData TemplateData) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	address := "localhost"
	port, err := freeport.GetFreePort()
	if err != nil {
		return
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", address, port))
	if err != nil {
		return
	}

	requestTimeout := 10 * time.Minute
	server := &http.Server{
		Handler:      buildRoutes(ctx, requestTimeout, templateData),
		ReadTimeout:  requestTimeout + 1*time.Minute,
		WriteTimeout: requestTimeout + 1*time.Minute,
	}
	defer server.Close()

	go func() {
		if err = server.Serve(listener); err != http.ErrServerClosed {
			cancel()
		}
	}()

	err = open.URL(fmt.Sprintf("http://%s:%d/data/storybook/?path=/story/universal-login--prompts", address, port))
	if err == nil {
		return
	}

	// Wait until the file is closed or input is cancelled
	<-ctx.Done()
}

func buildRoutes(ctx context.Context, requestTimeout time.Duration, templateData TemplateData) *http.ServeMux {
	router := http.NewServeMux()

	// Long polling waiting for file changes
	broadcaster := broadcastCustomTemplateChanges(ctx, templateData.Filename)
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
		http.ServeFile(w, r, templateData.Filename)
	})

	jstmpl := template.Must(template.New("tenant-data.js").Funcs(template.FuncMap{
		"asJS": func(v interface{}) string {
			a, _ := json.Marshal(v)
			return string(a)
		},
	}).ParseFS(tenantDataAsset, "data/tenant-data.js"))

	router.HandleFunc("/dynamic/tenant-data", func(w http.ResponseWriter, r *http.Request) {
		err := jstmpl.Execute(w, templateData)
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

	dir, file := filepath.Split(filename)
	c := make(chan notify.EventInfo)
	if err := notify.Watch(dir, c, notify.Write); err != nil {
		return publisher
	}

	go func() {
		for eventInfo := range c {
			if filepath.Base(eventInfo.Path()) == file {
				publisher.Pub(true)
			}
		}
	}()

	// release resources when the file is closed or the input is cancelled
	go func() {
		<-ctx.Done()
		notify.Stop(c)
		close(c)
	}()

	return publisher
}

func DefaultTemplate() string {
	return defaultTemplate
}

func FooterTemplate() string {
	return footerTemplate
}

func ImageTemplate() string {
	return imageTemplate
}
