package cli

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/auth0/go-auth0/management"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"nhooyr.io/websocket"

	"github.com/auth0/auth0-cli/internal/ansi"
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

			if err := ansi.Spinner("Waiting for changes", func() error {
				return startWebSocketServer(ctx)
			}); err != nil {
				return err
			}

			cli.renderer.Infof("Branding Updated")

			return nil
		},
	}

	return cmd
}

func startWebSocketServer(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:8000")
	if err != nil {
		return err
	}
	defer listener.Close()

	server := &http.Server{
		Handler: &webSocketHandler{
			cancel: cancel,
		},
		ReadTimeout:  time.Minute * 10,
		WriteTimeout: time.Minute * 10,
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Serve(listener)
	}()

	if err := browser.OpenURL("http://localhost:63342/auth0-cli/internal/cli/universal_login_customize.html?_ijt=up36ifofvbb0t6dtkn3j162ajb&_ij_reload=RELOAD_ON_SAVE"); err != nil {
		return err
	}

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return server.Close()
	}
}

type pageData struct {
	Text string `json:"text"`
}

type receivedSaveMessage struct {
	Templates management.BrandingUniversalLogin `json:"templates"`
	Themes    management.BrandingTheme          `json:"themes"`
}

type webSocketHandler struct {
	cancel context.CancelFunc
}

func (h *webSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost:*"},
	})
	if err != nil {
		log.Printf("error accepting WebSocket connection: %v", err)
		return
	}

	data := pageData{
		Text: "hello",
	}
	bytes, err := json.Marshal(&data)
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
	var msg receivedSaveMessage
	_, message, err := connection.Read(r.Context())
	if err != nil {
		log.Printf("error reading from WebSocket: %v", err)
		return
	}

	err = json.Unmarshal(message, &msg)
	if err != nil {
		log.Printf("failed to unmarshal message: %v", err)
		return
	}

	log.Printf("received message: %+v", msg)

	err = connection.Close(websocket.StatusNormalClosure, "Received save message")
	if err != nil {
		log.Printf("error closing WebSocket: %v", err)
	}

	h.cancel()
}
