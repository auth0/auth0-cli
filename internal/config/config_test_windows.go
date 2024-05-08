//go:build windows

package config

import "testing"

const ErrFileIsADirectory = "Incorrect function."

func FailsToSaveToReadOnlyDirectory(t *testing.T) {
	t.SkipNow()
}
