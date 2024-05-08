$GO_PACKAGES = go list $PSScriptRoot\..\... | Select-String -NotMatch @("vendor", "tools", "mock")
go test -v $GO_PACKAGES -coverprofile="coverage-unit-tests.out"