module github.com/auth0/auth0-cli

go 1.14

require (
	github.com/benbjohnson/clock v1.1.0 // indirect
	github.com/briandowns/spinner v1.11.1
	github.com/fatih/color v1.9.0 // indirect
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/mattn/go-colorable v0.1.6 // indirect
	github.com/olekukonko/tablewriter v0.0.4
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.5.1
	github.com/tidwall/pretty v1.0.2
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	gopkg.in/auth0.v5 v5.8.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v2 v2.2.8 // indirect
)

// replace gopkg.in/auth0.v5 => ../auth0
replace gopkg.in/auth0.v5 => github.com/go-auth0/auth0 v1.3.1-0.20210125203113-388ed60f4d87
