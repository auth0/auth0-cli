package setup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/poppy/config"
)

func boolPtr(b bool) *bool { return &b }

func TestFlagsParseWebUnscoped(t *testing.T) {
	tests := []struct {
		name  string
		given *bool
		want  *bool
	}{
		{name: "not passed", given: nil, want: nil},
		{name: "true", given: boolPtr(true), want: boolPtr(true)},
		{name: "false", given: boolPtr(false), want: boolPtr(false)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			in, err := Flags{WebUnscoped: test.given}.Parse()

			require.NoError(t, err)
			assert.Equal(t, test.want, in.WebUnscoped)
		})
	}

	t.Run("the inputs do not share the flag value", func(t *testing.T) {
		given := true
		in, err := Flags{WebUnscoped: &given}.Parse()
		require.NoError(t, err)

		given = false
		assert.True(t, *in.WebUnscoped)
	})
}

func TestStateAsksWebUnscoped(t *testing.T) {
	withFile := func(unscoped, set bool) *State {
		state := configuredState(t)
		state.File.Web.Unscoped, state.File.Web.UnscopedSet = unscoped, set

		return state
	}

	tests := []struct {
		name  string
		state *State
		in    Inputs
		want  bool
	}{
		{name: "no file", state: &State{}, want: true},
		{name: "the file has no value", state: withFile(false, false), want: true},
		{name: "the file says false", state: withFile(false, true), want: false},
		{name: "the file says true", state: withFile(true, true), want: false},
		{name: "the flag was passed as true", state: withFile(false, false), in: Inputs{WebUnscoped: boolPtr(true)}, want: false},
		{name: "the flag was passed as false", state: &State{}, in: Inputs{WebUnscoped: boolPtr(false)}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, test.state.AsksWebUnscoped(test.in))
		})
	}
}

func TestDeriveWebUnscoped(t *testing.T) {
	tests := []struct {
		name        string
		fileSet     bool
		fileValue   bool
		input       *bool
		wantUnscope bool
		wantSet     bool
	}{
		{name: "absent stays absent", wantUnscope: false, wantSet: false},
		{name: "the file's false is kept", fileSet: true, wantSet: true},
		{name: "the file's true is kept", fileSet: true, fileValue: true, wantUnscope: true, wantSet: true},
		{name: "a given true", input: boolPtr(true), wantUnscope: true, wantSet: true},
		{name: "a given false", input: boolPtr(false), wantSet: true},
		{name: "a given false over the file's true", fileSet: true, fileValue: true, input: boolPtr(false), wantSet: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, in := configuredState(t), configuredInputs()
			state.File.Web.Unscoped, state.File.Web.UnscopedSet = test.fileValue, test.fileSet
			in.WebUnscoped = test.input

			cfg, err := Derive(state, in)

			require.NoError(t, err)
			assert.Equal(t, test.wantUnscope, cfg.Web.Unscoped)
			assert.Equal(t, test.wantSet, cfg.Web.UnscopedSet)
		})
	}
}

func TestBuildWebUnscopedChangesTheFile(t *testing.T) {
	t.Run("only the answer differs, so the plan writes the file", func(t *testing.T) {
		state, in := configuredState(t), configuredInputs()
		in.WebUnscoped = boolPtr(true)

		plan, err := Build(state, in)

		require.NoError(t, err)
		assert.Equal(t, []Kind{KindFile}, kinds(plan))
	})

	t.Run("the answer the file already has changes nothing", func(t *testing.T) {
		state, in := configuredState(t), configuredInputs()
		state.File.Web.Unscoped, state.File.Web.UnscopedSet = true, true
		in.WebUnscoped = boolPtr(true)

		plan, err := Build(state, in)

		require.NoError(t, err)
		assert.True(t, plan.Empty())
	})

	t.Run("false is written when the file has no value", func(t *testing.T) {
		state, in := configuredState(t), configuredInputs()
		in.WebUnscoped = boolPtr(false)

		plan, err := Build(state, in)

		require.NoError(t, err)
		assert.Equal(t, []Kind{KindFile}, kinds(plan))
	})
}

func TestCommandWebUnscoped(t *testing.T) {
	base := Inputs{CompanyDomain: "shop.example", APIs: []string{shopAPI}, App: "app1", Out: config.DefaultFileName}

	tests := []struct {
		name  string
		given *bool
		want  string
	}{
		{name: "not given", want: `auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app app1`},
		{name: "true", given: boolPtr(true), want: `auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app app1 --web-unscoped=true`},
		{name: "false", given: boolPtr(false), want: `auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app app1 --web-unscoped=false`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			in := base
			in.WebUnscoped = test.given

			assert.Equal(t, test.want, FormatCommand("", Command(in, CommandOptions{}), 0))
		})
	}
}
