package proxy

import (
	"testing"

	"ant-chrome/backend/internal/config"
	"gopkg.in/yaml.v3"
)

func TestBuildExternalChainBridgeConfigRewritesDialerProxy(t *testing.T) {
	chain := ResolvedChain{Hops: []config.BrowserProxy{
		{ProxyId: "root", ProxyConfig: "name: root\ntype: direct\ndialer-proxy: relay"},
		{ProxyId: "relay", ProxyConfig: "name: relay\ntype: direct"},
	}}
	data, err := buildExternalChainBridgeConfig(chain, 19090)
	if err != nil {
		t.Fatalf("build config failed: %v", err)
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config failed: %v", err)
	}
	groups, _ := cfg["proxy-groups"].([]any)
	if len(groups) != 1 {
		t.Fatalf("unexpected groups: %d", len(groups))
	}
	group, _ := groups[0].(map[string]any)
	proxies, _ := group["proxies"].([]any)
	if len(proxies) != 1 || proxies[0] != "root" {
		t.Fatalf("unexpected entry proxies: %#v", proxies)
	}
	nodes, _ := cfg["proxies"].([]any)
	if len(nodes) != 2 {
		t.Fatalf("unexpected nodes: %d", len(nodes))
	}
	root, _ := nodes[0].(map[string]any)
	if root["dialer-proxy"] != "relay" {
		t.Fatalf("unexpected root dialer-proxy: %#v", root["dialer-proxy"])
	}
	relay, _ := nodes[1].(map[string]any)
	if _, exists := relay["dialer-proxy"]; exists {
		t.Fatalf("last hop should not keep dialer-proxy: %#v", relay)
	}
	dns, _ := cfg["dns"].(map[string]any)
	if dns["enhanced-mode"] != "redir-host" {
		t.Fatalf("unexpected dns enhanced-mode: %#v", dns["enhanced-mode"])
	}
}
