package proxy

import (
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestValidateProxyConfigAllowsResolvedChain(t *testing.T) {
	proxies := []config.BrowserProxy{
		{ProxyId: "mid-us", ProxyConfig: "socks5://127.0.0.1:2101", SourceID: "src-us", SourceNodeName: "US Relay 01"},
		{ProxyId: "res-us", ProxyConfig: "", SourceID: "src-us", SourceNodeName: "1.ss-ResidentialIDC-US2", RawProxyConfig: `type: ss
name: 1.ss-ResidentialIDC-US2
server: 1.1.1.1
port: 443
cipher: aes-128-gcm
password: pwd
dialer-proxy: US Relay 01`, UpstreamAlias: "US Relay 01", ChainMode: "chained"},
	}
	ok, msg := ValidateProxyConfig("", proxies, "res-us")
	if !ok {
		t.Fatalf("expected chain-aware validation to pass, msg=%s", msg)
	}
}

func TestValidateProxyConfigRejectsBrokenResolvedChain(t *testing.T) {
	proxies := []config.BrowserProxy{{
		ProxyId: "res-us", ProxyConfig: "", SourceID: "src-us", SourceNodeName: "1.ss-ResidentialIDC-US2", RawProxyConfig: `type: ss
name: 1.ss-ResidentialIDC-US2
server: 1.1.1.1
port: 443
cipher: aes-128-gcm
password: pwd
dialer-proxy: missing-upstream`, UpstreamAlias: "missing-upstream", ChainMode: "chained",
	}}
	ok, msg := ValidateProxyConfig("", proxies, "res-us")
	if ok {
		t.Fatalf("expected broken chain validation to fail, msg=%s", msg)
	}
}

func TestRequiresBridgeReturnsTrueForResolvedChain(t *testing.T) {
	proxies := []config.BrowserProxy{
		{ProxyId: "mid-us", ProxyConfig: "socks5://127.0.0.1:2101", SourceID: "src-us", SourceNodeName: "US Relay 01"},
		{ProxyId: "res-us", ProxyConfig: "", SourceID: "src-us", SourceNodeName: "1.ss-ResidentialIDC-US2", RawProxyConfig: `type: ss
name: 1.ss-ResidentialIDC-US2
server: 1.1.1.1
port: 443
cipher: aes-128-gcm
password: pwd
dialer-proxy: US Relay 01`, UpstreamAlias: "US Relay 01", ChainMode: "chained"},
	}
	if !RequiresBridge("", proxies, "res-us") {
		t.Fatalf("expected resolved chain to require local bridge")
	}
}
