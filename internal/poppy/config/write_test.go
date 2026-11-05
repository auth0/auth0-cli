package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dirEntries(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestWrite(t *testing.T) {
	t.Run("invalid config is refused and the file is untouched", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, DefaultFileName)
		require.NoError(t, os.WriteFile(path, []byte("original"), 0o644))

		err := Write(path, &Config{})

		var verr *ValidationError
		require.True(t, errors.As(err, &verr))
		assert.Equal(t, path, verr.File)

		got, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, "original", string(got))
		assert.Equal(t, []string{DefaultFileName}, dirEntries(t, dir))
	})

	t.Run("invalid config does not create the file", func(t *testing.T) {
		dir := t.TempDir()

		require.Error(t, Write(filepath.Join(dir, DefaultFileName), &Config{}))
		assert.Empty(t, dirEntries(t, dir))
	})

	t.Run("failed rename returns the error and leaves no temp file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, DefaultFileName)

		// A file cannot be renamed over a directory that is not empty.
		require.NoError(t, os.Mkdir(path, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(path, "keep"), []byte("original"), 0o644))

		err := Write(path, exampleConfig())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to replace")

		assert.Equal(t, []string{DefaultFileName}, dirEntries(t, dir))
		got, readErr := os.ReadFile(filepath.Join(path, "keep"))
		require.NoError(t, readErr)
		assert.Equal(t, "original", string(got))
	})

	t.Run("missing parent directory is an error and nothing is created", func(t *testing.T) {
		dir := t.TempDir()

		err := Write(filepath.Join(dir, "missing", DefaultFileName), exampleConfig())
		require.Error(t, err)
		assert.Empty(t, dirEntries(t, dir))
	})

	t.Run("replaces an existing file and leaves only that file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, DefaultFileName)
		require.NoError(t, os.WriteFile(path, []byte("old"), 0o644))

		require.NoError(t, Write(path, exampleConfig()))

		cfg, err := Read(path)
		require.NoError(t, err)
		assert.Equal(t, exampleConfig(), cfg)
		assert.Equal(t, []string{DefaultFileName}, dirEntries(t, dir))
	})

	t.Run("a path that cannot be inspected is an error", func(t *testing.T) {
		dir := t.TempDir()
		blocker := filepath.Join(dir, "file")
		require.NoError(t, os.WriteFile(blocker, nil, 0o644))

		err := Write(filepath.Join(blocker, DefaultFileName), exampleConfig())
		require.Error(t, err)
	})

	t.Run("file modes", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("file modes are not meaningful on windows")
		}

		tests := []struct {
			name     string
			existing os.FileMode
			want     os.FileMode
		}{
			{name: "new file gets 0644", want: 0o644},
			{name: "existing 0600 is preserved", existing: 0o600, want: 0o600},
			{name: "existing 0664 is preserved", existing: 0o664, want: 0o664},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), DefaultFileName)
				if tt.existing != 0 {
					require.NoError(t, os.WriteFile(path, []byte("old"), tt.existing))
					require.NoError(t, os.Chmod(path, tt.existing))
				}

				require.NoError(t, Write(path, exampleConfig()))

				info, err := os.Stat(path)
				require.NoError(t, err)
				assert.Equal(t, tt.want, info.Mode().Perm())
			})
		}
	})
}
