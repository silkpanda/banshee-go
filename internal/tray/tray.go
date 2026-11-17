package tray

import (
	"sync"

	"github.com/energye/systray"
)

// Actions defines callbacks triggered by the Windows system tray icon and menu.
type Actions struct {
	OnShow func()
	OnQuit func()
}

// Controller manages the running system tray icon and relay status item.
type Controller struct {
	mu         sync.Mutex
	statusItem *systray.MenuItem
	ready      bool
	pending    string
}

// Start launches the system tray loop on a dedicated goroutine.
func Start(icon []byte, actions Actions) *Controller {
	c := &Controller{pending: "Relay: Standby"}

	go systray.Run(func() {
		if len(icon) > 0 {
			systray.SetIcon(icon)
		}
		systray.SetTitle("Banshee")
		systray.SetTooltip("Banshee — Relay Standby")

		c.mu.Lock()
		c.statusItem = systray.AddMenuItem(c.pending, "Current Turing relay connection status")
		c.statusItem.Disable()
		c.ready = true
		c.mu.Unlock()

		systray.AddSeparator()
		showItem := systray.AddMenuItem("Show Banshee", "Reveal the Banshee command window")
		quitItem := systray.AddMenuItem("Quit Banshee", "Exit Banshee completely")

		showItem.Click(func() {
			if actions.OnShow != nil {
				actions.OnShow()
			}
		})
		quitItem.Click(func() {
			if actions.OnQuit != nil {
				actions.OnQuit()
			}
			systray.Quit()
		})

		systray.SetOnClick(func(_ systray.IMenu) {
			if actions.OnShow != nil {
				actions.OnShow()
			}
		})
		systray.SetOnDClick(func(_ systray.IMenu) {
			if actions.OnShow != nil {
				actions.OnShow()
			}
		})
	}, func() {})

	return c
}

// SetRelayStatus updates the disabled status line and hover tooltip in the tray.
func (c *Controller) SetRelayStatus(label string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending = "Relay: " + label
	if c.ready && c.statusItem != nil {
		c.statusItem.SetTitle(c.pending)
		systray.SetTooltip("Banshee — " + c.pending)
	}
}

// Stop tears down the system tray icon cleanly on shutdown.
func (c *Controller) Stop() {
	systray.Quit()
}
