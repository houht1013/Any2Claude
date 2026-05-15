//go:build !windows

package main

// runSystray is a no-op on non-Windows platforms.
// The process stays alive via signal.Notify in main.go.
func runSystray(proxyPort, dashPort int) {
	// Block forever on non-Windows (process kept alive by main)
	select {}
}
