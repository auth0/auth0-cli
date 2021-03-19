module github.com/auth0/auth0-cli

go 1.16

require (
	github.com/AlecAivazis/survey/v2 v2.2.8
	github.com/andybalholm/brotli v1.0.1 // indirect
	github.com/benbjohnson/clock v1.1.0 // indirect
	github.com/briandowns/spinner v1.12.0
	github.com/fatih/color v1.10.0 // indirect
	github.com/golang/mock v1.5.0
	github.com/golang/snappy v0.0.3 // indirect
	github.com/google/go-cmp v0.5.5
	github.com/klauspost/compress v1.11.9 // indirect
	github.com/klauspost/pgzip v1.2.5 // indirect
	github.com/lestrrat-go/jwx v1.1.4
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/mattn/go-isatty v0.0.12
	github.com/mattn/go-runewidth v0.0.10 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/mholt/archiver/v3 v3.5.0
	github.com/olekukonko/tablewriter v0.0.5
	github.com/pierrec/lz4/v4 v4.1.3 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/tidwall/pretty v1.1.0
	github.com/ulikunitz/xz v0.5.10 // indirect
	github.com/zalando/go-keyring v0.1.1
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83 // indirect
	golang.org/x/sys v0.0.0-20210319071255-635bc2c9138d // indirect
	golang.org/x/term v0.0.0-20210317153231-de623e64d2a6
	golang.org/x/text v0.3.5 // indirect
	gopkg.in/auth0.v5 v5.11.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

// replace gopkg.in/auth0.v5 => ../auth0

replace gopkg.in/auth0.v5 => github.com/go-auth0/auth0 v1.3.1-0.20210128024326-898cafab69ba
