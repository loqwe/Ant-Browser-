package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ant-chrome/backend/internal/config"
)

func TestMihomoBridgeDirectHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	manager := NewMihomoManager(config.DefaultConfig(), t.TempDir())
	defer manager.StopAll()
	socksURL, err := manager.EnsureBridge("name: direct-node\ntype: direct")
	if err != nil {
		t.Fatalf("ensure bridge failed: %v", err)
	}
	client, err := buildSocks5HTTPClient(strings.TrimPrefix(socksURL, "socks5://"), 5*time.Second)
	if err != nil {
		t.Fatalf("build client failed: %v", err)
	}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}

func TestMihomoChainBridgeDirectHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	manager := NewMihomoManager(config.DefaultConfig(), t.TempDir())
	defer manager.StopAll()
	chain := ResolvedChain{Hops: []config.BrowserProxy{
		{ProxyId: "root", ProxyConfig: "name: root\ntype: direct\ndialer-proxy: relay"},
		{ProxyId: "relay", ProxyConfig: "name: relay\ntype: direct"},
	}}
	socksURL, err := manager.EnsureChainBridge(chain)
	if err != nil {
		t.Fatalf("ensure chain bridge failed: %v", err)
	}
	client, err := buildSocks5HTTPClient(strings.TrimPrefix(socksURL, "socks5://"), 5*time.Second)
	if err != nil {
		t.Fatalf("build client failed: %v", err)
	}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}
