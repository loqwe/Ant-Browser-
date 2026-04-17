package backend

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
)

type BrowserFingerprintSuggestion struct {
	Seed                string `json:"seed"`
	Brand               string `json:"brand"`
	Platform            string `json:"platform"`
	Lang                string `json:"lang"`
	Timezone            string `json:"timezone"`
	Resolution          string `json:"resolution"`
	ColorDepth          string `json:"colorDepth"`
	HardwareConcurrency string `json:"hardwareConcurrency"`
	DeviceMemory        string `json:"deviceMemory"`
	TouchPoints         string `json:"touchPoints"`
	CanvasNoise         bool   `json:"canvasNoise"`
	WebGLVendor         string `json:"webglVendor"`
	WebGLRenderer       string `json:"webglRenderer"`
	AudioNoise          bool   `json:"audioNoise"`
	Fonts               string `json:"fonts"`
	WebRTCPolicy        string `json:"webrtcPolicy"`
	DoNotTrack          bool   `json:"doNotTrack"`
	MediaDevices        string `json:"mediaDevices"`
}

var fingerprintFixedTemplate = BrowserFingerprintSuggestion{
	Brand:               "Chrome",
	Platform:            "windows",
	Resolution:          "2560,1440",
	ColorDepth:          "24",
	HardwareConcurrency: "16",
	DeviceMemory:        "16",
	TouchPoints:         "0",
	CanvasNoise:         true,
	AudioNoise:          true,
	Fonts:               "Arial,Helvetica,Times New Roman,Courier New,Verdana",
	WebRTCPolicy:        "disable_non_proxied_udp",
	DoNotTrack:          false,
	MediaDevices:        "2,1,1",
}

var webglRendererPool = map[string][]string{
	"NVIDIA": {"NVIDIA GeForce RTX 3080", "NVIDIA GeForce RTX 3060", "NVIDIA GeForce GTX 1660", "NVIDIA GeForce GTX 1080 Ti"},
	"AMD":    {"AMD Radeon RX 6600", "AMD Radeon RX 580", "AMD Radeon Vega 8"},
	"Intel":  {"Intel(R) UHD Graphics 630", "Intel(R) UHD Graphics 620", "Intel(R) HD Graphics 520", "Intel(R) Iris(R) Xe Graphics"},
}

func (a *App) BrowserFingerprintSuggestByProxy(proxyId string, seed string) (BrowserFingerprintSuggestion, error) {
	proxyId = strings.TrimSpace(proxyId)
	seed = strings.TrimSpace(seed)
	if proxyId == "" {
		return BrowserFingerprintSuggestion{}, fmt.Errorf("?? ID ????")
	}
	if seed == "" {
		return BrowserFingerprintSuggestion{}, fmt.Errorf("????????")
	}
	var target *BrowserProxy
	for _, item := range a.getLatestProxies() {
		if strings.EqualFold(item.ProxyId, proxyId) {
			copy := item
			target = &copy
			break
		}
	}
	if target == nil {
		return BrowserFingerprintSuggestion{}, fmt.Errorf("?????: %s", proxyId)
	}
	health, ok := parseCachedProxyIPHealth(target.LastIPHealthJSON)
	if !ok {
		health = a.BrowserProxyCheckIPHealth(proxyId)
	}
	if !health.Ok {
		return BrowserFingerprintSuggestion{}, fmt.Errorf("????? IP ????: %s", strings.TrimSpace(health.Error))
	}
	return buildFingerprintSuggestion(seed, health), nil
}

func parseCachedProxyIPHealth(raw string) (ProxyIPHealthResult, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ProxyIPHealthResult{}, false
	}
	var result ProxyIPHealthResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return ProxyIPHealthResult{}, false
	}
	return result, result.Ok
}

func buildFingerprintSuggestion(seed string, health ProxyIPHealthResult) BrowserFingerprintSuggestion {
	countryCode := strings.ToUpper(strings.TrimSpace(mapRawString(health.RawData, "countryCode")))
	if countryCode == "" {
		countryCode = fallbackCountryCode(strings.TrimSpace(health.Country))
	}
	vendorList := []string{"NVIDIA", "AMD", "Intel"}
	vendor := vendorList[stableIndex(seed, "webgl-vendor", len(vendorList))]
	renderers := webglRendererPool[vendor]
	renderer := renderers[stableIndex(seed, vendor, len(renderers))]
	timezone := strings.TrimSpace(mapRawString(health.RawData, "timezone"))
	if timezone == "" {
		timezone = fallbackTimezone(countryCode)
	}
	suggestion := fingerprintFixedTemplate
	suggestion.Seed = seed
	suggestion.Lang = fallbackLanguage(countryCode)
	suggestion.Timezone = timezone
	suggestion.WebGLVendor = vendor
	suggestion.WebGLRenderer = renderer
	return suggestion
}

func stableIndex(seed string, namespace string, size int) int {
	if size <= 0 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	_, _ = h.Write([]byte("::"))
	_, _ = h.Write([]byte(namespace))
	return int(h.Sum32() % uint32(size))
}

func mapRawString(raw map[string]interface{}, key string) string {
	if raw == nil {
		return ""
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func fallbackCountryCode(country string) string {
	switch strings.ToLower(strings.TrimSpace(country)) {
	case "united states":
		return "US"
	case "japan":
		return "JP"
	case "china", "hong kong sar china", "hong kong":
		return "CN"
	case "south korea", "korea":
		return "KR"
	case "france":
		return "FR"
	case "germany":
		return "DE"
	case "united kingdom":
		return "GB"
	default:
		return ""
	}
}

func fallbackLanguage(countryCode string) string {
	switch countryCode {
	case "CN", "HK", "TW", "SG":
		return "zh-CN"
	case "JP":
		return "ja-JP"
	case "KR":
		return "ko-KR"
	case "FR":
		return "fr-FR"
	case "DE":
		return "de-DE"
	case "GB":
		return "en-GB"
	default:
		return "en-US"
	}
}

func fallbackTimezone(countryCode string) string {
	switch countryCode {
	case "CN", "HK", "TW", "SG":
		return "Asia/Shanghai"
	case "JP":
		return "Asia/Tokyo"
	case "KR":
		return "Asia/Seoul"
	case "GB":
		return "Europe/London"
	case "FR":
		return "Europe/Paris"
	case "DE":
		return "Europe/Berlin"
	case "BR":
		return "America/Sao_Paulo"
	case "AU":
		return "Australia/Sydney"
	case "CA":
		return "America/Toronto"
	default:
		return "America/New_York"
	}
}
