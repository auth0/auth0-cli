//go:build !windows

package authutil

const ErrBindFailure = "listen tcp 127.0.0.1:1234: bind: address already in use"
