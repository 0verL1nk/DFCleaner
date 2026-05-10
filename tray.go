package main

import (
	"os"

	"fyne.io/systray"
	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startTray() {
	systray.Run(func() {
		systray.SetIcon(trayIconData)
		systray.SetTitle("")
		systray.SetTooltip("DFCleaner - AI Disk Cleaner")

		mShow := systray.AddMenuItem("Show DFCleaner", "Show the main window")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Quit the application")

		go func() {
			for {
				select {
				case <-mShow.ClickedCh:
					wailsrt.WindowShow(a.ctx)
				case <-mQuit.ClickedCh:
					a.quitting = true
					systray.Quit()
				}
			}
		}()
	}, func() {
		if a.quitting {
			os.Exit(0)
		}
	})
}

func (a *App) stopTray() {
	a.quitting = true
	systray.Quit()
}
