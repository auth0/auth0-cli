package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// Read loads and validates the config file at path. A missing file returns an
// error satisfying errors.Is(err, fs.ErrNotExist). A file that parses but
// breaks the format rules returns a *ValidationError.
func Read(path string) (*Config, error) {
	cfg, err := ReadLenient(path)
	if err != nil {
		return nil, err
	}

	if violations := cfg.Validate(); len(violations) > 0 {
		return nil, &ValidationError{File: path, Violations: violations}
	}

	return cfg, nil
}

// ReadLenient loads the config file at path without validating it, so a stale
// or incomplete file can still be inspected. Syntax errors and values of the
// wrong type are still rejected.
func ReadLenient(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	cfg, err := decode(data)
	switch {
	case errors.Is(err, errTrailingContent):
		return nil, fmt.Errorf("failed to parse %s: unexpected content after the JSON document", path)
	case err != nil:
		return nil, decodeError(path, data, err)
	}

	return cfg, nil
}

func decodeError(path string, data []byte, err error) error {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError

	switch {
	case errors.Is(err, io.EOF):
		return fmt.Errorf("failed to parse %s: the file is empty", path)
	case errors.As(err, &syntaxErr):
		line, col := position(data, syntaxErr.Offset)
		return fmt.Errorf("failed to parse %s: invalid JSON at line %d, column %d: %w", path, line, col, err)
	case errors.As(err, &typeErr):
		line, col := position(data, typeErr.Offset)
		return fmt.Errorf("failed to parse %s: wrong type at line %d, column %d: %w", path, line, col, err)
	}

	return fmt.Errorf("failed to parse %s: %w", path, err)
}

// position converts a byte offset into a 1-based line and column.
func position(data []byte, offset int64) (line, col int) {
	offset = min(offset, int64(len(data)))

	line, col = 1, 1
	for _, b := range data[:offset] {
		if b == '\n' {
			line++
			col = 1
			continue
		}
		col++
	}

	return line, col
}
