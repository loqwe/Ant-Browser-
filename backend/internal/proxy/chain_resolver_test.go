package proxy

import (
	"strings"
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestResolveProxyChainExactDialerProxy(t *testing.T) {
	resolver := NewChainResolver([]config.BrowserProxy{
		{ProxyId: "mid-us", ProxyName: "US Relay 01", SourceID: "src-us", SourceNodeName: "US Relay 01", ProxyConfig: "socks5://127.0.0.1:2101"},
		{ProxyId: "res-us", ProxyName: "1.ss-ResidentialIDC-US2", SourceID: "src-us", SourceNodeName: "1.ss-ResidentialIDC-US2", UpstreamAlias: "US Relay 01", ChainMode: "chained"},
	})

	chain, err := resolver.ResolveProxyChain("res-us")
	if err != nil {
		t.Fatal(err)
	}
	if len(chain.Hops) != 2 || chain.Hops[1].ProxyId != "mid-us" {
		t.Fatalf("unexpected chain: %#v", chain.Hops)
	}
}

func TestResolveProxyChainMatchesDisplayGroupAlias(t *testing.T) {
	resolver := NewChainResolver([]config.BrowserProxy{
		{ProxyId: "mid-us", ProxyName: "US Relay 01", SourceID: "src-us", SourceNodeName: "US Relay 01", DisplayGroup: "US", RawProxyGroupName: "US", ProxyConfig: "socks5://127.0.0.1:2101"},
		{ProxyId: "res-us", ProxyName: "1.ss-ResidentialIDC-US2", SourceID: "src-us", SourceNodeName: "1.ss-ResidentialIDC-US2", UpstreamAlias: "US", ChainMode: "chained"},
	})

	chain, err := resolver.ResolveProxyChain("res-us")
	if err != nil {
		t.Fatal(err)
	}
	if len(chain.Hops) != 2 || chain.Hops[1].ProxyId != "mid-us" {
		t.Fatalf("unexpected chain: %#v", chain.Hops)
	}
}

func TestResolveProxyChainDetectsCycle(t *testing.T) {
	resolver := NewChainResolver([]config.BrowserProxy{
		{ProxyId: "p1", SourceID: "src-us", SourceNodeName: "A", UpstreamAlias: "B", ChainMode: "chained"},
		{ProxyId: "p2", SourceID: "src-us", SourceNodeName: "B", UpstreamAlias: "A", ChainMode: "chained"},
	})

	_, err := resolver.ResolveProxyChain("p1")
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got=%v", err)
	}
}

func TestResolveProxyChainBrokenWhenUpstreamMissing(t *testing.T) {
	resolver := NewChainResolver([]config.BrowserProxy{{ProxyId: "p1", SourceID: "src-us", SourceNodeName: "A", UpstreamAlias: "missing", ChainMode: "chained"}})

	chain, err := resolver.ResolveProxyChain("p1")
	if err == nil || !strings.Contains(err.Error(), "missing upstream") {
		t.Fatalf("expected missing upstream error, got=%v", err)
	}
	if chain.Status != "broken" || len(chain.Hops) != 1 {
		t.Fatalf("unexpected broken chain: %#v", chain)
	}
}
