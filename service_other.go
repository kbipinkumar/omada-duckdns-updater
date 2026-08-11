//go:build !windows

package main

import "fmt"

// runMaybeAsService runs the application in the foreground on non-Windows platforms.
func runMaybeAsService() error {
	return runConsole()
}

// handleServiceCommand rejects Windows-only service management flags on other OSes.
func handleServiceCommand(cmd string) error {
	return fmt.Errorf("service management (-service %s) is only supported on Windows", cmd)
}
