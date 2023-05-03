package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/auth0/go-auth0/management"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"nhooyr.io/websocket"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
)

func universalLoginCustomizeBranding(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "customize",
		Args:  cobra.NoArgs,
		Short: "Customize the entire Universal Login Experience",
		Long:  "Customize and preview changes to the Universal Login Experience.",
		Example: `  auth0 universal-login customize
  auth0 ul customize`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			var pageData *pageData
			if err := ansi.Spinner("Gathering data", func() (err error) {
				pageData, err = fetchPageData(ctx, cli.api)
				return err
			}); err != nil {
				return err
			}

			var receivedMessage *receivedSaveMessage
			if err := ansi.Waiting(func() (err error) {
				receivedMessage, err = startWebSocketServer(ctx, pageData)
				return err
			}); err != nil {
				return err
			}

			cli.renderer.Infof("%+v", receivedMessage)

			if err := ansi.Waiting(func() error {
				return cli.api.Branding.SetUniversalLogin(
					&management.BrandingUniversalLogin{
						Body: receivedMessage.Templates.Body,
					},
				)
			}); err != nil {
				return fmt.Errorf("failed to update the template for the New Universal Login Experience: %w", err)
			}

			return nil
		},
	}

	return cmd
}

func startWebSocketServer(ctx context.Context, pageData *pageData) (*receivedSaveMessage, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:8000")
	if err != nil {
		return nil, err
	}
	defer listener.Close()

	handler := &webSocketHandler{
		cancel:   cancel,
		pageData: pageData,
	}

	server := &http.Server{
		Handler:      handler,
		ReadTimeout:  time.Minute * 10,
		WriteTimeout: time.Minute * 10,
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Serve(listener)
	}()

	if err := browser.OpenURL("http://localhost:63342/auth0-cli/internal/cli/universal_login_customize.html?_ijt=up36ifofvbb0t6dtkn3j162ajb&_ij_reload=RELOAD_ON_SAVE"); err != nil {
		return nil, err
	}

	select {
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return handler.receivedSaveMessage, server.Close()
	}
}

type pageData struct {
	AuthenticationProfile *management.Prompt                 `json:"authentication_profile"`
	Branding              *management.Branding               `json:"branding"`
	Templates             *management.BrandingUniversalLogin `json:"templates"`
	Themes                *management.BrandingTheme          `json:"themes"`
	Tenant                *management.Tenant                 `json:"tenant"`
}

func fetchPageData(ctx context.Context, api *auth0.API) (*pageData, error) {
	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() (err error) {
		return ensureCustomDomainIsEnabled(ctx, api)
	})

	var authenticationProfile *management.Prompt
	group.Go(func() (err error) {
		authenticationProfile, err = api.Prompt.Read()
		return err
	})

	var brandingSettings *management.Branding
	group.Go(func() (err error) {
		brandingSettings = fetchBrandingSettingsOrUseDefaults(ctx, api)
		return nil
	})

	var currentTemplate *management.BrandingUniversalLogin
	group.Go(func() (err error) {
		currentTemplate = fetchBrandingTemplateOrUseEmpty(ctx, api)
		return nil
	})

	var currentTheme *management.BrandingTheme
	group.Go(func() (err error) {
		currentTheme = fetchBrandingThemeOrUseEmpty(ctx, api)
		return nil
	})

	var tenant *management.Tenant
	group.Go(func() (err error) {
		tenant, err = api.Tenant.Read(management.Context(ctx))
		return err
	})

	if err := group.Wait(); err != nil {
		return nil, err
	}

	data := &pageData{
		AuthenticationProfile: authenticationProfile,
		Branding:              brandingSettings,
		Templates:             currentTemplate,
		Themes:                currentTheme,
		Tenant:                tenant,
	}

	return data, nil
}

func fetchBrandingThemeOrUseEmpty(ctx context.Context, api *auth0.API) *management.BrandingTheme {
	currentTheme, err := api.BrandingTheme.Default(management.Context(ctx))
	if err != nil {
		currentTheme = &management.BrandingTheme{}
	}

	return currentTheme
}

type receivedSaveMessage struct {
	Templates *management.BrandingUniversalLogin `json:"templates"`
	Themes    *management.BrandingTheme          `json:"themes"`
}

type webSocketHandler struct {
	receivedSaveMessage *receivedSaveMessage
	cancel              context.CancelFunc
	pageData            *pageData
}

func (h *webSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost:*"},
	})
	if err != nil {
		log.Printf("error accepting WebSocket connection: %v", err)
		return
	}

	bytes, err := json.Marshal(&h.pageData)
	if err != nil {
		log.Printf("failed to marshal message: %v", err)
		return
	}

	err = connection.Write(r.Context(), websocket.MessageText, bytes)
	if err != nil {
		log.Printf("failed to write message: %v", err)
		h.cancel()
		return
	}

	// Just wait for the save button, no need to wait for more messages.
	_, message, err := connection.Read(r.Context())
	if err != nil {
		log.Printf("error reading from WebSocket: %v", err)
		return
	}

	var msg receivedSaveMessage
	err = json.Unmarshal(message, &msg)
	if err != nil {
		log.Printf("failed to unmarshal message: %v", err)
		return
	}

	h.receivedSaveMessage = &msg

	err = connection.Close(websocket.StatusNormalClosure, "Received save message")
	if err != nil {
		log.Printf("error closing WebSocket: %v", err)
	}

	h.cancel()
}
