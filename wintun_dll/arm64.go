//go:build windows && arm64

package main

import _ "embed"

//go:embed wintun_arm64.dll
var wintunDLL []byte
