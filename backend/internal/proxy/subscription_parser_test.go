package proxy

import "testing"

func TestParseSubscriptionDocumentPreservesChainMetadata(t *testing.T) {
	raw := []byte(`proxies:
  - name: 1.ss-ResidentialIDC-US2
    type: ss
    server: 1.1.1.1
    port: 443
    cipher: aes-128-gcm
    password: pwd
    dialer-proxy: us-relay
proxy-groups:
  - name: US
    type: select
    proxies:
      - 1.ss-ResidentialIDC-US2
`)

	doc, err := ParseSubscriptionDocument(raw, "src-us", "https://example.com/sub")
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Nodes) != 1 {
		t.Fatalf("unexpected nodes: %#v", doc.Nodes)
	}

	node := doc.Nodes[0]
	if node.SourceNodeName != "1.ss-ResidentialIDC-US2" {
		t.Fatalf("unexpected source node name: %#v", node)
	}
	if node.UpstreamAlias != "us-relay" {
		t.Fatalf("unexpected upstream alias: %#v", node)
	}
	if node.DisplayGroup != "US" || node.RawProxyGroupName != "US" {
		t.Fatalf("unexpected display group: %#v", node)
	}
	if node.RawProxyConfig == "" {
		t.Fatalf("expected raw proxy config to be preserved: %#v", node)
	}
}

func TestParseSubscriptionDocumentUsesRawNodeAsProxyConfig(t *testing.T) {
	raw := []byte(`proxies:
  - name: hk-01
    type: http
    server: hk.example.com
    port: 443
    username: user
    password: pwd
`)

	doc, err := ParseSubscriptionDocument(raw, "src-raw", "https://example.com/sub")
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Nodes) != 1 {
		t.Fatalf("unexpected nodes: %#v", doc.Nodes)
	}
	if doc.Nodes[0].ProxyConfig != doc.Nodes[0].RawProxyConfig {
		t.Fatalf("expected proxy config to keep raw node, got=%q raw=%q", doc.Nodes[0].ProxyConfig, doc.Nodes[0].RawProxyConfig)
	}
}
