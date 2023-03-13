package users

import (
	_ "embed"
)

var (
	// EmptyExample for the user import options.
	//go:embed data/empty-example.json
	EmptyExample string

	// BasicExample for the user import options.
	//go:embed data/basic-example.json
	BasicExample string

	// CustomPasswordHashExample for the user import options.
	//go:embed data/custom-password-hash-example.json
	CustomPasswordHashExample string

	// MFAFactors for the user import options.
	//go:embed data/mfa-factors.json
	MFAFactors string
)
