package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArgumentGetters(t *testing.T) {
	t.Run("returns the name", func(t *testing.T) {
		argument := Argument{Name: "Foo"}
		want := "Foo"
		got := argument.GetName()

		assert.Equal(t, want, got)
	})

	t.Run("returns the label", func(t *testing.T) {
		argument := Argument{Name: "Foo"}
		want := "Foo:"
		got := argument.GetLabel()

		assert.Equal(t, want, got)
	})

	t.Run("returns the help", func(t *testing.T) {
		argument := Argument{Help: "Foo"}
		want := "Foo"
		got := argument.GetHelp()

		assert.Equal(t, want, got)
	})

	t.Run("returns that the argument is required", func(t *testing.T) {
		argument := Argument{}
		want := true
		got := argument.GetIsRequired()

		assert.Equal(t, want, got)
	})
}
