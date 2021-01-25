// module gopkg.in/auth0.v5
module github.com/turcottedanny/auth0

go 1.12

require (
	github.com/PuerkitoBio/rehttp v1.0.0
	golang.org/x/oauth2 v0.0.0-20200902213428-5d25da1a8d43
	gopkg.in/auth0.v5 v5.8.0
)

replace gopkg.in/auth0.v5 => ./
