package proxy

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	bridgeServerResolveCache sync.Map
	clashFakeIPPrefix        = netip.MustParsePrefix("198.18.0.0/15")
)

type dohAnswer struct {
	Data string `json:"data"`
	Type int    `json:"type"`
}

type dohResponse struct {
	Answer []dohAnswer `json:"Answer"`
}

func resolveBridgeServerHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return host
	}
	if ip := net.ParseIP(host); ip != nil {
		return host
	}
	if cached, ok := bridgeServerResolveCache.Load(host); ok {
		if resolved, ok := cached.(string); ok && resolved != "" {
			return resolved
		}
	}
	if resolved := firstUsableLookupIP(host); resolved != "" {
		bridgeServerResolveCache.Store(host, resolved)
		return resolved
	}
	if resolved := resolveViaCloudflareDoH(host); resolved != "" {
		bridgeServerResolveCache.Store(host, resolved)
		return resolved
	}
	return host
}

func firstUsableLookupIP(host string) string {
	ips, err := net.LookupIP(host)
	if err != nil {
		return ""
	}
	for _, ip := range ips {
		if ip == nil || ip.To4() == nil {
			continue
		}
		addr, err := netip.ParseAddr(ip.String())
		if err != nil || clashFakeIPPrefix.Contains(addr) {
			continue
		}
		return ip.String()
	}
	return ""
}

func resolveViaCloudflareDoH(host string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	endpoint := "https://cloudflare-dns.com/dns-query?name=" + url.QueryEscape(host) + "&type=A"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("accept", "application/dns-json")
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, "1.1.1.1:443")
		},
		TLSClientConfig: &tls.Config{ServerName: "cloudflare-dns.com"},
	}
	client := &http.Client{Transport: transport, Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var payload dohResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ""
	}
	for _, answer := range payload.Answer {
		ip := strings.TrimSpace(answer.Data)
		if answer.Type != 1 || net.ParseIP(ip) == nil {
			continue
		}
		if addr, err := netip.ParseAddr(ip); err == nil && !clashFakeIPPrefix.Contains(addr) {
			return ip
		}
	}
	return ""
}
