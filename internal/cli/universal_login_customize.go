package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"nhooyr.io/websocket"
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

			if err := startWebSocketServer(ctx); err != nil {
				return fmt.Errorf("server error: %w", err)
			}

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
		Handler:      &webSocketHandler{},
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
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
		return nil
	}
}

type message struct {
	Text string `json:"text"`
}

type webSocketHandler struct {
}

func (h *webSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost:*"},
	})
	if err != nil {
		log.Printf("error accepting WebSocket connection: %v", err)
		return
	}

	defer func() {
		err := connection.Close(websocket.StatusNormalClosure, "the sky is falling")
		if err != nil {
			log.Printf("error closing WebSocket: %v", err)
		}
	}()

	// Just wait for the save button, no need to wait for more messages.
	var msg message
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

	log.Printf("received message: %s", msg.Text)
}
