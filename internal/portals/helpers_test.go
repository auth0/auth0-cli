package portals_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// wrapped adds context to an error, as the code around a store does.
func wrapped(err error) error {
	return fmt.Errorf("check the permissions: %w", err)
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()

	data, err := json.Marshal(value)
	require.NoError(t, err)

	return string(data)
}
