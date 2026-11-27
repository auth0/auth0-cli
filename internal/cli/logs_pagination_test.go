package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
)

func TestGetLatestLogsPagination(t *testing.T) {
	for _, number := range []int{50, 100, 150, 250, 1000} {
		t.Run(strconv.Itoa(number), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				page, err := strconv.Atoi(r.URL.Query().Get("page"))
				require.NoError(t, err)
				size, err := strconv.Atoi(r.URL.Query().Get("per_page"))
				require.NoError(t, err)
				logs := make([]map[string]string, 0, size)
				for i := page * size; i < (page+1)*size; i++ {
					logs = append(logs, map[string]string{"log_id": fmt.Sprint(i)})
				}
				w.Header().Set("Content-Type", "application/json")
				require.NoError(t, json.NewEncoder(w).Encode(logs))
			}))
			defer server.Close()

			api, err := management.New(server.URL, management.WithInsecure(), management.WithStaticToken("test-token"))
			require.NoError(t, err)
			logs, err := getLatestLogs(context.Background(), &cli{api: auth0.NewAPI(api)}, number, "")
			require.NoError(t, err)
			require.Len(t, logs, number)
			for i, log := range logs {
				require.Equal(t, strconv.Itoa(i), log.GetLogID())
			}
		})
	}
}
