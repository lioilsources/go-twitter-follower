package main

import (
	_ "embed"
	"fmt"
	"time"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/trayicon.png
var trayIcon []byte

func (a *App) initSystray() {
	systray.Run(a.onTrayReady, func() {})
}

func (a *App) onTrayReady() {
	systray.SetIcon(trayIcon)
	systray.SetTooltip("Twitter Following Tracker")

	mShow := systray.AddMenuItem("Show Dashboard", "Open the main window")
	systray.AddSeparator()
	mFetch := systray.AddMenuItem("Fetch Now", "Fetch following for all accounts")
	a.mu.Lock()
	a.trayLastFetchItem = systray.AddMenuItem("Last fetch: --", "")
	a.trayLastFetchItem.Disable()
	a.mu.Unlock()
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit the application")

	// Handle menu clicks
	mShow.Click(func() {
		runtime.WindowShow(a.ctx)
	})

	mFetch.Click(func() {
		go func() {
			a.FetchAllAccounts()
		}()
	})

	mQuit.Click(func() {
		systray.Quit()
		runtime.Quit(a.ctx)
	})

	// Also show window when tray icon is clicked
	systray.SetOnClick(func(menu systray.IMenu) {
		runtime.WindowShow(a.ctx)
	})
}

func (a *App) updateTrayLastFetch() {
	a.mu.Lock()
	item := a.trayLastFetchItem
	a.mu.Unlock()

	if item != nil {
		item.SetTitle(fmt.Sprintf("Last fetch: %s", time.Now().Format("15:04")))
	}
}
