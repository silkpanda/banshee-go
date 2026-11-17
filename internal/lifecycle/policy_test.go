package lifecycle

import "testing"

func TestLifecycleHideVsQuit(t *testing.T) {
	p := NewPolicy()

	if !p.ShouldPreventClose() {
		t.Fatal("expected closing window to hide to tray by default")
	}
	if p.IsQuitting() {
		t.Fatal("expected IsQuitting false initially")
	}

	p.RequestQuit()

	if p.ShouldPreventClose() {
		t.Fatal("expected closing to be allowed after RequestQuit")
	}
	if !p.IsQuitting() {
		t.Fatal("expected IsQuitting true after RequestQuit")
	}
}
