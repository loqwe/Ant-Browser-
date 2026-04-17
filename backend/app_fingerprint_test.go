package backend

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
)

func TestBuildFingerprintSuggestionUsesPreciseTimezone(t *testing.T) {
	s := buildFingerprintSuggestion("12345", ProxyIPHealthResult{
		Ok:      true,
		Country: "United States",
		RawData: map[string]interface{}{
			"countryCode": "US",
			"timezone":    "America/Chicago",
		},
	})
	if s.Brand != "Chrome" || s.Platform != "windows" {
		t.Fatalf("unexpected fixed identity: %#v", s)
	}
	if s.Lang != "en-US" || s.Timezone != "America/Chicago" {
		t.Fatalf("expected precise timezone + lang mapping: %#v", s)
	}
}

func TestBuildFingerprintSuggestionFallsBackByCountry(t *testing.T) {
	s := buildFingerprintSuggestion("12345", ProxyIPHealthResult{
		Ok:      true,
		Country: "Japan",
		RawData: map[string]interface{}{
			"countryCode": "JP",
		},
	})
	if s.Lang != "ja-JP" || s.Timezone != "Asia/Tokyo" {
		t.Fatalf("expected country fallback mapping: %#v", s)
	}
}


func TestBuildFingerprintSuggestionAppliesFixedTemplateFields(t *testing.T) {
	s := buildFingerprintSuggestion("seed-fixed", ProxyIPHealthResult{
		Ok:      true,
		Country: "United States",
		RawData: map[string]interface{}{
			"countryCode": "US",
			"timezone":    "America/New_York",
		},
	})
	if s.Resolution != "2560,1440" || s.ColorDepth != "24" {
		t.Fatalf("expected fixed display template: %#v", s)
	}
	if s.HardwareConcurrency != "16" || s.DeviceMemory != "16" || s.TouchPoints != "0" {
		t.Fatalf("expected fixed hardware template: %#v", s)
	}
	if !s.CanvasNoise || !s.AudioNoise {
		t.Fatalf("expected fixed noise switches enabled: %#v", s)
	}
	if s.WebRTCPolicy != "disable_non_proxied_udp" || s.DoNotTrack {
		t.Fatalf("expected fixed privacy template: %#v", s)
	}
	if s.MediaDevices != "2,1,1" || s.Fonts != "Arial,Helvetica,Times New Roman,Courier New,Verdana" {
		t.Fatalf("expected fixed media/font template: %#v", s)
	}
}

func TestBrowserFingerprintSuggestByProxyUsesCachedIPHealth(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "ant.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	app := &App{config: cfg}
	app.browserMgr = browser.NewManager(cfg, "")
	app.browserMgr.ProxyDAO = browser.NewSQLiteProxyDAO(db.GetConn())
	cached, _ := json.Marshal(ProxyIPHealthResult{ProxyId: "p1", Ok: true, Country: "United States", RawData: map[string]interface{}{"countryCode": "US", "timezone": "America/Chicago"}})
	if err := app.browserMgr.ProxyDAO.Upsert(config.BrowserProxy{ProxyId: "p1", ProxyName: "node-1", ProxyConfig: "http://127.0.0.1:7890"}); err != nil {
		t.Fatal(err)
	}
	if err := app.browserMgr.ProxyDAO.UpdateIPHealthResult("p1", string(cached)); err != nil {
		t.Fatal(err)
	}
	s, err := app.BrowserFingerprintSuggestByProxy("p1", "12345")
	if err != nil {
		t.Fatal(err)
	}
	if s.Seed != "12345" || s.Brand != "Chrome" || s.Platform != "windows" {
		t.Fatalf("unexpected suggestion: %#v", s)
	}
	if s.Lang != "en-US" || s.Timezone != "America/Chicago" {
		t.Fatalf("unexpected locale suggestion: %#v", s)
	}
	if s.WebGLVendor == "" || s.WebGLRenderer == "" {
		t.Fatalf("expected webgl suggestion: %#v", s)
	}
}
