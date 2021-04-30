package branding

import (
	"embed"
	_ "embed"
)

var (
	//go:embed data/storybook/*
	templatePreviewAssets embed.FS

	//go:embed data/tenant-data.js
	tenantDataAsset string

	//go:embed data/default-template.liquid
	DefaultTemplate string

	//go:embed data/footer-template.liquid
	FooterTemplate string

	//go:embed data/image-template.liquid
	ImageTemplate string
)
