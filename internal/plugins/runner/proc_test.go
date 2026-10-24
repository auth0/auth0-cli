package runner

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code uint32
		want int
	}{
		{"success", 0, 0},
		{"Ctrl-C is 128 plus SIGINT", 0xC000013A, 130},
		{"an ordinary failure is kept", 7, 7},
		{"the largest code that fits in an int on every system is kept", 0x7FFFFFFF, 0x7FFFFFFF},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, mapExitCode(test.code))
		})
	}
}

func TestMapExitCode_OnlyCtrlCIsMapped(t *testing.T) {
	t.Parallel()

	assert.NotEqual(t, 130, mapExitCode(controlCExit-1))
	assert.NotEqual(t, 130, mapExitCode(controlCExit+1))
}

func TestJobLingering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		active uint32
		err    error
		want   bool
	}{
		{"nothing is left", 0, nil, false},
		{"one process is left", 1, nil, true},
		{"a query that failed counts as lingering", 0, errors.New("failed"), true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, jobLingering(test.active, test.err))
		})
	}
}
