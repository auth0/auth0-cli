package validators

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestNoArgs(t *testing.T) {
	c := &cobra.Command{Use: "c"}
	args := []string{}

	result := NoArgs(c, args)
	require.Nil(t, result)
}

func TestNoArgsWithArgs(t *testing.T) {
	c := &cobra.Command{Use: "c"}
	args := []string{"foo"}

	result := NoArgs(c, args)
	require.EqualError(t, result, "`c` does not take any positional arguments. See `c --help` for supported flags and usage")
}

func TestExactArgs(t *testing.T) {
	c := &cobra.Command{Use: "c"}
	args := []string{"foo"}

	result := ExactArgs("<name>")(c, args)
	require.Nil(t, result)
}

func TestExactArgsTooMany(t *testing.T) {
	c := &cobra.Command{Use: "c"}
	args := []string{"foo", "bar"}

	result := ExactArgs("<name>")(c, args)
	require.EqualError(t, result, "`c` requires <name>. See `c --help` for supported flags and usage")
}

func TestExactArgsTooManyMoreThan1(t *testing.T) {
	c := &cobra.Command{Use: "c"}
	args := []string{"foo", "bar", "baz"}

	result := ExactArgs("<key>", "<value>")(c, args)
	require.EqualError(t, result, "`c` requires <key> <value>. See `c --help` for supported flags and usage")
}
