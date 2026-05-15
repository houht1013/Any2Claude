package main

import (
	_ "embed"
)

//go:embed assets/any2claude-tray-logo.ico
var embeddedICO []byte

// buildIcon returns the embedded ICO file data.
func buildIcon() []byte {
	return embeddedICO
}
