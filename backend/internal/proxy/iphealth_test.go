package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchIPHealthInfoWithClientFallsBackToIPAPI(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(120 * time.Millisecond)
	}))
	defer slow.Close()

	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ip":"1.2.3.4","country_name":"United States","country_code":"US","region":"California","city":"Los Angeles","org":"Test ISP","timezone":"America/Los_Angeles"}`))
	}))
	defer fallback.Close()

	client := &http.Client{Timeout: 40 * time.Millisecond}
	data, err := fetchIPHealthInfoWithClient(client, []ipHealthEndpoint{
		{Source: "ippure", URL: slow.URL},
		{Source: "ipapi", URL: fallback.URL},
	})
	if err != nil {
		t.Fatalf("expected fallback source to succeed: %v", err)
	}
	if data["_source"] != "ipapi" {
		t.Fatalf("expected fallback source ipapi, got=%v", data["_source"])
	}
	if data["ip"] != "1.2.3.4" || data["country"] != "United States" {
		t.Fatalf("unexpected normalized data: %#v", data)
	}
	if data["countryCode"] != "US" || data["timezone"] != "America/Los_Angeles" {
		t.Fatalf("expected canonical country/timezone fields: %#v", data)
	}
}
