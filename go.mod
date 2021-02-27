module github.com/auth0/auth0-cli

go 1.16

require (
	github.com/AlecAivazis/survey/v2 v2.2.7
	github.com/benbjohnson/clock v1.1.0 // indirect
	github.com/briandowns/spinner v1.12.0
	github.com/fatih/color v1.10.0 // indirect
	github.com/golang/mock v1.4.4
	github.com/google/go-cmp v0.5.4
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/mattn/go-isatty v0.0.12
	github.com/mattn/go-runewidth v0.0.10 // indirect
	github.com/mholt/archiver/v3 v3.5.0
	github.com/olekukonko/tablewriter v0.0.4
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.5.1
	github.com/tidwall/pretty v1.0.2
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/sys v0.0.0-20210124154548-22da62e12c0c // indirect
	gopkg.in/auth0.v5 v5.8.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v2 v2.2.8
)

// replace gopkg.in/auth0.v5 => ../auth0

replace gopkg.in/auth0.v5 => github.com/go-auth0/auth0 v1.3.1-0.20210128024326-898cafab69ba
