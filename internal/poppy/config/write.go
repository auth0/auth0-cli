package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// defaultFileMode is the mode of a newly created config file.
const defaultFileMode fs.FileMode = 0o644

// marshal encodes the config as the file content: two-space indent, a trailing
// newline, the member order of the protocol, no HTML escaping, and [] and {}
// instead of null for empty lists and scopes. It does not validate.
func marshal(c *Config) ([]byte, error) {
	compact, err := c.encode()
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := json.Indent(&buf, compact, "", "  "); err != nil {
		return nil, fmt.Errorf("failed to encode config: %w", err)
	}

	buf.WriteByte('\n')

	return buf.Bytes(), nil
}

// Write validates the config and writes it to path atomically: the content
// goes to a temporary file in the same directory which is then renamed over
// path. On any failure the original file is left untouched. Parent directories
// are not created.
func Write(path string, c *Config) error {
	if violations := c.Validate(); len(violations) > 0 {
		return &ValidationError{File: path, Violations: violations}
	}

	data, err := marshal(c)
	if err != nil {
		return err
	}

	mode := defaultFileMode
	info, err := os.Stat(path)
	switch {
	case err == nil:
		mode = info.Mode().Perm()
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("failed to inspect %s: %w", path, err)
	}

	tmpName, err := writeTemp(path, data, mode)
	if err != nil {
		return err
	}

	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("failed to replace %s: %w", path, err)
	}

	return nil
}

// writeTemp writes data to a new temporary file next to path, with the given
// mode, and returns its name. The file is removed again on failure.
func writeTemp(path string, data []byte, mode fs.FileMode) (string, error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create a temporary file for %s: %w", path, err)
	}

	err = tmp.Chmod(mode)
	if err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("failed to write %s: %w", tmp.Name(), err)
	}

	return tmp.Name(), nil
}
