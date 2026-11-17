package lifecycle

import "sync/atomic"

// Policy controls whether closing the main window hides to tray or exits the process.
type Policy struct {
	quitting atomic.Bool
}

// NewPolicy returns a Policy in normal tray-resident mode.
func NewPolicy() *Policy {
	return &Policy{}
}

// RequestQuit marks the application as intentionally exiting (e.g. from the tray Quit menu).
func (p *Policy) RequestQuit() {
	p.quitting.Store(true)
}

// IsQuitting reports whether an explicit quit was requested.
func (p *Policy) IsQuitting() bool {
	return p.quitting.Load()
}

// ShouldPreventClose returns true when closing the window should hide to tray instead of quitting.
func (p *Policy) ShouldPreventClose() bool {
	return !p.quitting.Load()
}
