package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUnifiedDelayTestReconnectsWhenWarmupConnectionCloses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	proxyInstance, err := buildMihomoProxy("name: direct-node\ntype: direct", nil)
	if err != nil {
		t.Fatalf("buildMihomoProxy failed: %v", err)
	}
	defer func() { _ = proxyInstance.Close() }()

	result := unifiedDelayTest("direct-node", proxyInstance, server.URL, 3*time.Second)
	if !result.Ok {
		t.Fatalf("unifiedDelayTest should reconnect after warmup close: %+v", result)
	}
	if result.LatencyMs < 0 {
		t.Fatalf("latency should not be negative: %+v", result)
	}
}

func TestUnifiedDelayTestFallsBackToFreshGetAfterHeadTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			time.Sleep(120 * time.Millisecond)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	proxyInstance, err := buildMihomoProxy("name: direct-node\ntype: direct", nil)
	if err != nil {
		t.Fatalf("buildMihomoProxy failed: %v", err)
	}
	defer func() { _ = proxyInstance.Close() }()

	result := unifiedDelayTest("direct-node", proxyInstance, server.URL, 80*time.Millisecond)
	if !result.Ok {
		t.Fatalf("unifiedDelayTest should retry GET with fresh context: %+v", result)
	}
}

func TestUnifiedDelayTestFallsBackToFreshGetAfterHeadTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			time.Sleep(120 * time.Millisecond)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	proxyInstance, err := buildMihomoProxy("name: direct-node\ntype: direct", nil)
	if err != nil {
		t.Fatalf("buildMihomoProxy failed: %v", err)
	}
	defer func() { _ = proxyInstance.Close() }()

	result := unifiedDelayTest("direct-node", proxyInstance, server.URL, 80*time.Millisecond)
	if !result.Ok {
		t.Fatalf("unifiedDelayTest should retry GET with fresh context: %+v", result)
	}
}
