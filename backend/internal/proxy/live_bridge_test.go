package proxy

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ant-chrome/backend/internal/config"
)

func TestLiveBridgeHTTPClient(t *testing.T) {
	if os.Getenv("ANT_BROWSER_RUN_LIVE_PROXY_TESTS") == "" {
		t.Skip("set ANT_BROWSER_RUN_LIVE_PROXY_TESTS=1 to run live proxy tests")
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.Browser.XrayBinaryPath = filepath.Join(repoRoot, "bin", "xray.exe")
	cfg.Browser.SingBoxBinaryPath = filepath.Join(repoRoot, "bin", "sing-box.exe")
	cfg.Browser.ClashBinaryPath = filepath.Join(repoRoot, "bin", "mihomo.exe")

	relayPorts := map[string]int{"01": 20074, "02": 20049, "03": 20052}
	proxies := make([]config.BrowserProxy, 0, 6)
	testIDs := make([]string, 0, 6)
	for suffix, port := range relayPorts {
		relayID := "relay-" + suffix
		relayName := fmt.Sprintf("hk-relay-%s", suffix)
		chainID := "res-us2-via-" + suffix
		proxies = append(proxies,
			config.BrowserProxy{ProxyId: relayID, ProxyName: relayName, ProxyConfig: fmt.Sprintf("name: \"%s\"\ntype: trojan\nserver: Ffyyvvq-b.catcat321.com\nport: %d\npassword: 50ff98be-b6d0-498b-b310-21749ffa4f94\nsni: HK.catxstar.com\nskip-cert-verify: true\nclient-fingerprint: chrome\nudp: true", relayName, port), SourceID: "src-live", SourceNodeName: relayName, ChainMode: "single", ChainStatus: "resolved"},
			config.BrowserProxy{ProxyId: chainID, ProxyName: "1.ss-ResidentialIDC-US2", ProxyConfig: "name: \"1.ss-ResidentialIDC-US2\"\ntype: ss\nserver: residentialidc-us1-node.rysonai.com\nport: 30982\ncipher: aes-256-gcm\npassword: piOMFuA2Q8t4063JnlcMdTLde4AoEOmXapci5cU7VVI=\nudp: true\ndialer-proxy: \"us-relay-selector\"", SourceID: "src-live", SourceNodeName: "1.ss-ResidentialIDC-US2", ChainMode: "chained", UpstreamAlias: "us-relay-selector", UpstreamProxyId: relayID, ChainStatus: "resolved"},
		)
		testIDs = append(testIDs, relayID, chainID)
	}

	xrayMgr := NewXrayManager(cfg, repoRoot)
	singboxMgr := NewSingBoxManager(cfg, repoRoot)
	mihomoMgr := NewMihomoManager(cfg, repoRoot)
	defer xrayMgr.StopAll()
	defer singboxMgr.StopAll()
	defer mihomoMgr.StopAll()

	for _, id := range testIDs {
		client, err := buildProxyHTTPClient("", id, proxies, xrayMgr, singboxMgr, mihomoMgr, 20*time.Second)
		if err != nil {
			t.Fatalf("%s build client failed: %v", id, err)
		}
		req, _ := http.NewRequest(http.MethodGet, "http://93.184.216.34/", nil)
		req.Host = "example.com"
		start := time.Now()
		resp, err := client.Do(req)
		elapsed := time.Since(start)
		if err != nil {
			t.Logf("%s request failed after %s: %v", id, elapsed, err)
			continue
		}
		resp.Body.Close()
		t.Logf("%s status=%d elapsed=%s", id, resp.StatusCode, elapsed)
	}
}
