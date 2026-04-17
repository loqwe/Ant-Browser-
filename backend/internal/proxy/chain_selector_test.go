package proxy

import (
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestResolveProxyChainUsesExplicitUpstreamProxyId(t *testing.T) {
	resolver := NewChainResolver([]config.BrowserProxy{
		{ProxyId: "relay-1", ProxyName: "Relay 01", SourceID: "src-us", SourceNodeName: "Relay 01", ProxyConfig: "socks5://127.0.0.1:2101"},
		{ProxyId: "relay-2", ProxyName: "Relay 02", SourceID: "src-us", SourceNodeName: "Relay 02", ProxyConfig: "socks5://127.0.0.1:2102"},
		{ProxyId: "res-us", ProxyName: "1.ss-ResidentialIDC-US2", SourceID: "src-us", SourceNodeName: "1.ss-ResidentialIDC-US2", UpstreamAlias: "US Relay", UpstreamProxyId: "relay-2", ChainMode: "chained"},
	})

	chain, err := resolver.ResolveProxyChain("res-us")
	if err != nil {
		t.Fatal(err)
	}
	if len(chain.Hops) != 2 || chain.Hops[1].ProxyId != "relay-2" {
		t.Fatalf("unexpected chain: %#v", chain.Hops)
	}
}
