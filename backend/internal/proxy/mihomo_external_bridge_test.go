package proxy

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestMihomoManagerPrefersExternalCoreBridge(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "fake-mihomo-core")
	if runtime.GOOS == "windows" {
		helper += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", helper, "./testdata/fake_mihomo_core.go")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake core failed: %v, %s", err, out)
	}
	cfg := config.DefaultConfig()
	cfg.Browser.ClashBinaryPath = helper
	manager := NewMihomoManager(cfg, t.TempDir())
	defer manager.StopAll()
	src := "name: direct-node\ntype: direct"
	socksURL, err := manager.EnsureBridge(src)
	if err != nil {
		t.Fatalf("ensure bridge failed: %v", err)
	}
	if socksURL == "" {
		t.Fatal("expected socks url")
	}
	key := computeNodeKey("mihomo:\x00" + src)
	bridge := manager.Bridges[key]
	if bridge == nil {
		t.Fatal("expected cached bridge")
	}
	if bridge.Backend != "external-core" {
		t.Fatalf("expected external-core backend, got %q", bridge.Backend)
	}
	if bridge.Cmd == nil || bridge.Cmd.Process == nil {
		t.Fatal("expected external core process")
	}
}
