package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPickerOptions(t *testing.T) {
	t.Run("returns the labels", func(t *testing.T) {
		options := pickerOptions{pickerOption{label: "Foo"}, pickerOption{label: "Bar"}}
		want := []string{"Foo", "Bar"}
		got := options.labels()

		assert.Equal(t, want, got)
	})

	t.Run("returns the default label", func(t *testing.T) {
		options := pickerOptions{pickerOption{label: "Foo"}, pickerOption{label: "Bar"}}
		want := "Foo"
		got := options.defaultLabel()

		assert.Equal(t, want, got)
	})

	t.Run("returns an empty label when there are no options", func(t *testing.T) {
		options := pickerOptions{}
		want := ""
		got := options.defaultLabel()

		assert.Equal(t, want, got)
	})

	t.Run("returns the value for a given label", func(t *testing.T) {
		options := pickerOptions{pickerOption{label: "Foo", value: "0"}, pickerOption{label: "Bar", value: "1"}}
		want := "1"
		got := options.getValue("Bar")

		assert.Equal(t, want, got)
	})

	t.Run("returns an empty value given a non-existent label", func(t *testing.T) {
		options := pickerOptions{pickerOption{label: "Foo"}}
		want := ""
		got := options.getValue("Bar")

		assert.Equal(t, want, got)
	})
}
