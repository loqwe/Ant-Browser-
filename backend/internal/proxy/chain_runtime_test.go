package proxy

import (
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestCompileXrayChainUsesTaggedOutbounds(t *testing.T) {
	chain := ResolvedChain{RootProxyID: "res-us", Status: "resolved", Hops: []config.BrowserProxy{
		{ProxyId: "res-us", ProxyConfig: `type: ss
name: res-us
server: 1.1.1.1
port: 443
cipher: aes-128-gcm
password: pwd`, DnsServers: "8.8.8.8"},
		{ProxyId: "mid-us", ProxyConfig: "socks5://127.0.0.1:2101"},
	}}

	runtime, err := CompileXrayChain(chain)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.EntryTag != "proxy-hop-0" || len(runtime.Outbounds) != 2 {
		t.Fatalf("unexpected runtime: %#v", runtime)
	}
	if runtime.Outbounds[0]["tag"] != "proxy-hop-0" {
		t.Fatalf("unexpected root tag: %#v", runtime.Outbounds[0])
	}
	streamSettings, ok := runtime.Outbounds[0]["streamSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected streamSettings, got=%#v", runtime.Outbounds[0])
	}
	sockopt, ok := streamSettings["sockopt"].(map[string]interface{})
	if !ok || sockopt["dialerProxy"] != "proxy-hop-1" {
		t.Fatalf("expected next-hop dialerProxy, got=%#v", runtime.Outbounds[0])
	}
}

func TestCompileSingBoxChainUsesDetour(t *testing.T) {
	chain := ResolvedChain{RootProxyID: "hy2-us", Status: "resolved", Hops: []config.BrowserProxy{
		{ProxyId: "hy2-us", ProxyConfig: "hysteria2://pwd@example.com:443?sni=example.com"},
		{ProxyId: "mid-us", ProxyConfig: "socks5://127.0.0.1:2101"},
	}}

	runtime, err := CompileSingBoxChain(chain)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.EntryTag != "proxy-hop-0" || len(runtime.Outbounds) != 2 {
		t.Fatalf("unexpected runtime: %#v", runtime)
	}
	if runtime.Outbounds[0]["detour"] != "proxy-hop-1" {
		t.Fatalf("expected detour to next hop, got=%#v", runtime.Outbounds[0])
	}
}
