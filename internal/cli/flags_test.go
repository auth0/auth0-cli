package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlagGetters(t *testing.T) {
	t.Run("returns the name", func(t *testing.T) {
		flag := Flag{Name: "Foo"}
		want := "Foo"
		got := flag.GetName()

		assert.Equal(t, want, got)
	})

	t.Run("returns the label", func(t *testing.T) {
		flag := Flag{Name: "Foo"}
		want := "Foo:"
		got := flag.GetLabel()

		assert.Equal(t, want, got)
	})

	t.Run("returns the help", func(t *testing.T) {
		flag := Flag{Help: "Foo"}
		want := "Foo"
		got := flag.GetHelp()

		assert.Equal(t, want, got)
	})

	t.Run("returns that the flag is required", func(t *testing.T) {
		flag := Flag{IsRequired: true}
		want := true
		got := flag.GetIsRequired()

		assert.Equal(t, want, got)
	})

	t.Run("returns that the flag is not required", func(t *testing.T) {
		flag := Flag{IsRequired: false}
		want := false
		got := flag.GetIsRequired()

		assert.Equal(t, want, got)
	})
}
