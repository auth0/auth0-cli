package users

import (
	_ "embed"
)

var (
	// EmptyExample ...
	//go:embed data/empty-example.json
	EmptyExample string

	// BasicExample ...
	//go:embed data/basic-example.json
	BasicExample string

	// CustomPasswordHashExample ...
	//go:embed data/custom-password-hash-example.json
	CustomPasswordHashExample string

	// MFAFactors ...
	//go:embed data/mfa-factors.json
	MFAFactors string
)
