package users

import (
	_ "embed"
)

var (
	//go:embed data/basic-example.json
	BasicExample string

	//go:embed data/custom-password-hash-example.json
	CustomPasswordHashExample string

	//go:embed data/mfa-factors.json
	MFAFactors string
)
