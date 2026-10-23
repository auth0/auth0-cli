package approvals_test

import (
	"errors"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/approvals"
)

func TestParsePins(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  approvals.Pins
	}{
		{name: "one pin", value: id + "@" + hashA, want: approvals.Pins{id: {hashA}}},
		{name: "spaces around entries", value: "  " + id + "@" + hashA + " ,\t" + "auth0/other@" + hashB + "  ", want: approvals.Pins{id: {hashA}, "auth0/other": {hashB}}},
		{name: "more than one hash for an ID", value: id + "@" + hashA + "," + id + "@" + hashB, want: approvals.Pins{id: {hashA, hashB}}},
		{name: "the same pin twice", value: id + "@" + hashA + "," + id + "@" + hashA, want: approvals.Pins{id: {hashA}}},
		{name: "empty value", value: "", want: nil},
		{name: "only spaces", value: "   ", want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pins, err := approvals.ParsePins(test.value)

			require.NoError(t, err)
			assert.Equal(t, test.want, pins)
		})
	}
}

func TestParsePinsRejectsMalformedEntries(t *testing.T) {
	tests := []struct {
		name  string
		value string
		entry string // The quoted entry that the error names, which is the whole value when empty.
	}{
		{name: "uppercase hex", value: id + "@sha256:" + upper},
		{name: "missing @", value: id + hashA},
		{name: "name without registry", value: "checkmate@" + hashA},
		{name: "uppercase ID", value: "Auth0/checkmate@" + hashA},
		{name: "truncated hash", value: id + "@" + hashA[:len(hashA)-1]},
		{name: "no hash prefix", value: id + "@" + hashA[len("sha256:"):]},
		{name: "trailing comma", value: id + "@" + hashA + ",", entry: `""`},
		{name: "one bad entry among good ones", value: id + "@" + hashA + ", nonsense", entry: `"nonsense"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pins, err := approvals.ParsePins(test.value)

			entry := test.entry
			if entry == "" {
				entry = strconv.Quote(test.value)
			}

			require.ErrorIs(t, err, approvals.ErrBadPins)
			assert.ErrorContains(t, err, entry)
			assert.Nil(t, pins)
		})
	}
}

func TestPins(t *testing.T) {
	t.Run("a pin approves on any tenant with no file, and writes none", func(t *testing.T) {
		a := newApprovals(t)
		a.Pins = approvals.Pins{id: {hashA}}

		for _, name := range []string{tenant, "other.eu.auth0.com"} {
			approved, err := a.Approved(name, id, hashA)

			require.NoError(t, err)
			assert.True(t, approved)
			require.NoError(t, a.Check(name, id, hashA))
		}

		_, err := os.Stat(a.Dir)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("a pin with another hash approves nothing", func(t *testing.T) {
		a := newApprovals(t)
		a.Pins = approvals.Pins{id: {hashB}, "auth0/other": {hashA}}

		approved, err := a.Approved(tenant, id, hashA)

		require.NoError(t, err)
		assert.False(t, approved)

		var notApproved *approvals.NotApprovedError

		require.True(t, errors.As(a.Check(tenant, id, hashA), &notApproved))
		assert.False(t, notApproved.Changed)
	})

	t.Run("a pin approves although the approvals file is damaged", func(t *testing.T) {
		a := newApprovals(t)
		a.Pins = approvals.Pins{id: {hashA}}
		writeFile(t, a, "not json")

		require.NoError(t, a.Check(tenant, id, hashA))
		require.ErrorIs(t, a.Check(tenant, id, hashB), approvals.ErrDamaged)
	})
}
