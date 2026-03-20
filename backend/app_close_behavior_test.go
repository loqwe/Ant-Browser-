package backend

import (
	"context"
	goruntime "runtime"
	"testing"
)

func TestPlatformSupportsTrayCloseFlowForOS(t *testing.T) {
	if !platformSupportsTrayCloseFlowForOS("windows") {
		t.Fatal("expected Windows to keep tray close flow enabled")
	}
	if platformSupportsTrayCloseFlowForOS("linux") {
		t.Fatal("expected Linux to skip tray close flow")
	}
}

func TestShouldBlockClose_NonWindowsDoesNotIntercept(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("Windows keeps the tray-based close confirmation flow")
	}

	app := NewApp("")
	if ShouldBlockClose(app, context.Background()) {
		t.Fatal("expected non-Windows close to proceed without interception")
	}
}
