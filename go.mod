module github.com/auth0/auth0-cli

go 1.14

require (
	github.com/AlecAivazis/survey/v2 v2.2.7
	github.com/benbjohnson/clock v1.1.0 // indirect
	github.com/briandowns/spinner v1.12.0
	github.com/fatih/color v1.9.0 // indirect
	github.com/golang/mock v1.4.4
	github.com/google/go-cmp v0.5.4
	github.com/jsanda/tablewriter v0.0.2-0.20190614032957-c4e45dc9c708
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/mattn/go-colorable v0.1.6 // indirect
	github.com/mattn/go-isatty v0.0.12
	github.com/mattn/go-runewidth v0.0.10 // indirect
	github.com/mholt/archiver/v3 v3.5.0
	github.com/rivo/uniseg v0.2.0 // indirect
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

replace gopkg.in/auth0.v5 => github.com/go-auth0/auth0 v1.3.1-0.20210127175916-f1d24c8c0f88
