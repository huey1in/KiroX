//go:build windows

package main

import (
	"sync"

	toast "git.sr.ht/~jackmordaunt/go-toast/v2"
)

var (
	desktopNotificationInitOnce sync.Once
	desktopNotificationInitErr  error
)

func showDesktopNotification(title, message string) error {
	desktopNotificationInitOnce.Do(func() {
		desktopNotificationInitErr = toast.SetAppData(toast.AppData{AppID: "KiroX"})
	})
	if desktopNotificationInitErr != nil {
		return desktopNotificationInitErr
	}

	notification := toast.Notification{
		AppID:    "KiroX",
		Title:    title,
		Body:     message,
		Audio:    toast.Silent,
		Duration: toast.Short,
	}
	return notification.Push()
}
