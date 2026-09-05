//go:build !windows

package main

func showDesktopNotification(_, _ string) error {
	return nil
}
