package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/metacubex/mihomo/adapter"
	C "github.com/metacubex/mihomo/constant"
	"gopkg.in/yaml.v3"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
)

// ─── Clash 标准测速 URL ───
// 使用 HTTP 与 Clash 客户端保持一致

var defaultTestURLs = []string{"http://www.gstatic.com/generate_204", "http://cp.cloudflare.com/generate_204", "https://www.gstatic.com/generate_204"}

// SpeedTestConfig 测速参数
type SpeedTestConfig struct {
	Timeout    time.Duration
	TCPTimeout time.Duration
	URLs       []string
}

var DefaultSpeedTestConfig = SpeedTestConfig{
	Timeout:    10 * time.Second,
	TCPTimeout: 5 * time.Second,
}

// ─── 对外入口 ───

// SpeedTest 使用 mihomo 代理适配器进行测速。
// 采用 unified-delay 策略：先建立连接（预热），再单独计时 HTTP 往返，
// 与 Clash 客户端 unified-delay: true 的延迟结果一致。
func SpeedTest(
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	mihomoMgr *MihomoManager,
	cfg *SpeedTestConfig,
) TestResult {
	log := logger.New("SpeedTest")
	if cfg == nil {
		copyCfg := DefaultSpeedTestConfig
		cfg = &copyCfg
	}
	src := ""
	for _, item := range proxies {
		if strings.EqualFold(item.ProxyId, proxyId) {
			src = strings.TrimSpace(item.ProxyConfig)
			break
		}
	}
	src, chain, chained, err := ResolveRuntimeChain(src, proxies, proxyId)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Error: err.Error()}
	}
	if strings.ToLower(src) == "direct://" {
		return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: 0}
	}
	testURLs := append([]string{}, defaultTestURLs...)
	if len(cfg.URLs) > 0 {
		testURLs = append([]string{}, cfg.URLs...)
	}
	if len(testURLs) == 0 {
		testURLs = []string{"http://www.gstatic.com/generate_204"}
	}
	if chained {
		if SupportsMihomoChain(chain) {
			if mihomoMgr != nil {
				socksURL, bridgeErr := mihomoMgr.EnsureChainBridge(chain)
				if bridgeErr != nil {
					return TestResult{ProxyId: proxyId, Ok: false, Error: bridgeErr.Error()}
				}
				mapping, mapErr := proxyConfigToMapping(socksURL)
				if mapErr != nil {
					return TestResult{ProxyId: proxyId, Ok: false, Error: mapErr.Error()}
				}
				proxyInstance, parseErr := adapter.ParseProxy(mapping)
				if parseErr != nil {
					return TestResult{ProxyId: proxyId, Ok: false, Error: parseErr.Error()}
				}
				return unifiedDelayTestWithFallback(proxyId, proxyInstance, testURLs, cfg.Timeout)
			}
			proxyInstance, buildErr := buildMihomoChainProxy(chain)
			if buildErr != nil {
				return TestResult{ProxyId: proxyId, Ok: false, Error: buildErr.Error()}
			}
			return unifiedDelayTestWithFallback(proxyId, proxyInstance, testURLs, cfg.Timeout)
		}
		var socksURL string
		if ChainUsesSingBox(chain) {
			if singboxMgr == nil {
				return TestResult{ProxyId: proxyId, Ok: false, Error: "sing-box manager unavailable"}
			}
			socksURL, err = singboxMgr.EnsureChainBridge(chain)
		} else {
			if xrayMgr == nil {
				return TestResult{ProxyId: proxyId, Ok: false, Error: "xray manager unavailable"}
			}
			socksURL, err = xrayMgr.EnsureChainBridge(chain)
		}
		if err != nil {
			return TestResult{ProxyId: proxyId, Ok: false, Error: err.Error()}
		}
		mapping, mapErr := proxyConfigToMapping(socksURL)
		if mapErr != nil {
			return TestResult{ProxyId: proxyId, Ok: false, Error: mapErr.Error()}
		}
		proxyInstance, parseErr := adapter.ParseProxy(mapping)
		if parseErr != nil {
			return TestResult{ProxyId: proxyId, Ok: false, Error: parseErr.Error()}
		}
		return unifiedDelayTestWithFallback(proxyId, proxyInstance, testURLs, cfg.Timeout)
	}
	if src == "" {
		return TestResult{ProxyId: proxyId, Ok: false, Error: "未找到代理节点"}
	}
	if SupportsMihomoBridge(src) {
		if mihomoMgr != nil {
			socksURL, bridgeErr := mihomoMgr.EnsureBridge(src)
			if bridgeErr != nil {
				return TestResult{ProxyId: proxyId, Ok: false, Error: bridgeErr.Error()}
			}
			mapping, mapErr := proxyConfigToMapping(socksURL)
			if mapErr != nil {
				return TestResult{ProxyId: proxyId, Ok: false, Error: mapErr.Error()}
			}
			proxyInstance, parseErr := adapter.ParseProxy(mapping)
			if parseErr != nil {
				return TestResult{ProxyId: proxyId, Ok: false, Error: parseErr.Error()}
			}
			return unifiedDelayTestWithFallback(proxyId, proxyInstance, testURLs, cfg.Timeout)
		}
		proxyInstance, buildErr := buildMihomoProxy(src, nil)
		if buildErr != nil {
			return TestResult{ProxyId: proxyId, Ok: false, Error: buildErr.Error()}
		}
		return unifiedDelayTestWithFallback(proxyId, proxyInstance, testURLs, cfg.Timeout)
	}
	mapping, err := proxyConfigToMapping(src)
	if err != nil {
		log.Warn("代理配置解析失败，降级到 TCP ping", logger.F("proxy_id", proxyId), logger.F("error", err.Error()))
		return tcpPingFallback(proxyId, src, cfg.TCPTimeout, log)
	}
	proxyInstance, err := adapter.ParseProxy(mapping)
	if err != nil {
		log.Warn("mihomo 代理创建失败，降级到 TCP ping", logger.F("proxy_id", proxyId), logger.F("error", err.Error()), logger.F("type", mapping["type"]))
		return tcpPingFallback(proxyId, src, cfg.TCPTimeout, log)
	}
	return unifiedDelayTestWithFallback(proxyId, proxyInstance, testURLs, cfg.Timeout)
}

// unifiedDelayTest 模拟 Clash unified-delay 模式：
// 1. 通过代理建立到目标的 TCP 连接（预热，不计入延迟）
// 2. 发送第一次 HTTP 请求预热连接（不计入延迟）
// 3. 在已建立的连接上发送第二次 HTTP 请求，只计这次的 RTT
// 这样测出的延迟 = 纯 HTTP 往返时间，和 Clash unified-delay: true 一致。
func unifiedDelayTestWithFallback(proxyId string, px C.Proxy, testURLs []string, timeout time.Duration) TestResult {
	var last TestResult
	for _, testURL := range testURLs {
		testURL = strings.TrimSpace(testURL)
		if testURL == "" {
			continue
		}
		last = unifiedDelayTest(proxyId, px, testURL, timeout)
		if last.Ok {
			return last
		}
	}
	if last.ProxyId == "" {
		return TestResult{ProxyId: proxyId, Ok: false, Error: "无可用测试 URL"}
	}
	dialResult := proxyDialFallback(proxyId, px, testURLs, timeout)
	if dialResult.Ok {
		return dialResult
	}
	if strings.TrimSpace(dialResult.Error) != "" {
		last.Error = "HTTP 探测失败： " + strings.TrimSpace(last.Error) + "；代理拨号失败： " + strings.TrimSpace(dialResult.Error)
	}
	return last
}

func proxyDialFallback(proxyId string, px C.Proxy, testURLs []string, timeout time.Duration) TestResult {
	var last TestResult
	for _, testURL := range testURLs {
		testURL = strings.TrimSpace(testURL)
		if testURL == "" {
			continue
		}
		addr, err := urlToMeta(testURL)
		if err != nil {
			last = TestResult{ProxyId: proxyId, Ok: false, Error: fmt.Sprintf("URL 解析失败： %v", err)}
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		start := time.Now()
		conn, err := px.DialContext(ctx, &addr)
		latency := time.Since(start).Milliseconds()
		cancel()
		if err != nil {
			last = TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency, Error: fmt.Sprintf("代理拨号失败： %v", err)}
			continue
		}
		_ = conn.Close()
		return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: latency}
	}
	if last.ProxyId == "" {
		return TestResult{ProxyId: proxyId, Ok: false, Error: "无可用测试 URL"}
	}
	return last
}

func unifiedDelayTest(proxyId string, px C.Proxy, testURL string, timeout time.Duration) TestResult {
	// 解析目标地址
	addr, err := urlToMeta(testURL)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Error: fmt.Sprintf("URL 解析失败: %v", err)}
	}

	// 步骤 1：构造通过代理拨号的 HTTP client
	transport := &http.Transport{
		DialContext: func(reqCtx context.Context, network, address string) (net.Conn, error) {
			conn, dialErr := px.DialContext(reqCtx, &addr)
			if dialErr != nil {
				return nil, fmt.Errorf("代理连接失败: %w", dialErr)
			}
			return conn, nil
		},
		DisableKeepAlives: false,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	defer client.CloseIdleConnections()

	// 步骤 2：第一次请求预热（不计时）
	newRequest := func(method string) (*http.Request, context.CancelFunc) {
		reqCtx, reqCancel := context.WithTimeout(context.Background(), timeout)
		req, _ := http.NewRequestWithContext(reqCtx, method, testURL, nil)
		return req, reqCancel
	}

	method := http.MethodHead
	req1, cancel1 := newRequest(method)
	resp1, err := client.Do(req1)
	cancel1()
	if err != nil {
		method = http.MethodGet
		req1, cancel1 = newRequest(method)
		resp1, err = client.Do(req1)
		cancel1()
	}
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Error: err.Error()}
	}
	resp1.Body.Close()

	// 步骤 3：第二次请求计时（纯 HTTP RTT）
	start := time.Now()
	req2, cancel2 := newRequest(method)
	resp2, err := client.Do(req2)
	latency := time.Since(start).Milliseconds()
	cancel2()

	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency, Error: err.Error()}
	}
	resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusNoContent {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency,
			Error: fmt.Sprintf("HTTP %d", resp2.StatusCode)}
	}

	return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: latency}
}

// urlToMeta 将 URL 转换为 mihomo Metadata
func urlToMeta(rawURL string) (C.Metadata, error) {
	var host string
	var portNum uint16
	if strings.HasPrefix(rawURL, "https://") {
		host = rawURL[len("https://"):]
		portNum = 443
	} else if strings.HasPrefix(rawURL, "http://") {
		host = rawURL[len("http://"):]
		portNum = 80
	} else {
		return C.Metadata{}, fmt.Errorf("不支持的 URL scheme")
	}
	// 去掉 path
	if idx := strings.Index(host, "/"); idx >= 0 {
		host = host[:idx]
	}
	// 检查是否有自定义端口
	if h, p, err := net.SplitHostPort(host); err == nil {
		host = h
		fmt.Sscanf(p, "%d", &portNum)
	}

	meta := C.Metadata{
		Host:    host,
		DstPort: portNum,
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		meta.DstIP = addr
	}
	return meta, nil
}

// ─── 代理配置转换为 mihomo mapping ───

func proxyConfigToMapping(src string) (map[string]any, error) {
	src = strings.TrimSpace(src)
	l := strings.ToLower(src)

	// http/https 直连代理
	if strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") {
		return parseStandardProxy(src, "http")
	}
	// socks5 直连代理
	if strings.HasPrefix(l, "socks5://") {
		return parseStandardProxy(src, "socks5")
	}

	// URI 格式（vmess:// vless:// 等）暂不支持直接转 mapping，降级
	if strings.Contains(l, "://") && !strings.Contains(l, "type:") {
		return nil, fmt.Errorf("URI 格式暂不支持: %s", l[:min(30, len(l))])
	}

	// Clash YAML 格式 → 直接解析
	return parseClashYAMLToMapping(src)
}

func parseStandardProxy(src string, proxyType string) (map[string]any, error) {
	rest := src[strings.Index(src, "://")+3:]

	var username, password, hostport string
	if atIdx := strings.LastIndex(rest, "@"); atIdx >= 0 {
		userInfo := rest[:atIdx]
		hostport = rest[atIdx+1:]
		parts := strings.SplitN(userInfo, ":", 2)
		username = parts[0]
		if len(parts) > 1 {
			password = parts[1]
		}
	} else {
		hostport = rest
	}
	hostport = strings.SplitN(hostport, "/", 2)[0]

	host, port := splitHostPort(hostport)
	if host == "" || port == 0 {
		return nil, fmt.Errorf("无法解析地址: %s", src)
	}

	mapping := map[string]any{
		"name":   "speedtest-proxy",
		"type":   proxyType,
		"server": host,
		"port":   port,
	}
	if username != "" {
		mapping["username"] = username
		mapping["password"] = password
	}
	return mapping, nil
}

func parseClashYAMLToMapping(src string) (map[string]any, error) {
	var payload interface{}
	if err := yaml.Unmarshal([]byte(src), &payload); err != nil {
		return nil, fmt.Errorf("YAML 解析失败: %v", err)
	}

	node := pickClashNode(payload)
	if node == nil {
		return nil, fmt.Errorf("无法提取 Clash 节点")
	}

	if _, ok := node["name"]; !ok {
		node["name"] = "speedtest-proxy"
	}

	return node, nil
}

func splitHostPort(hostport string) (string, int) {
	if strings.HasPrefix(hostport, "[") {
		if idx := strings.LastIndex(hostport, "]:"); idx >= 0 {
			host := hostport[1:idx]
			port := 0
			fmt.Sscanf(hostport[idx+2:], "%d", &port)
			return host, port
		}
		return strings.Trim(hostport, "[]"), 0
	}
	idx := strings.LastIndex(hostport, ":")
	if idx < 0 {
		return hostport, 0
	}
	host := hostport[:idx]
	port := 0
	fmt.Sscanf(hostport[idx+1:], "%d", &port)
	return host, port
}

// ─── TCP Ping 降级 ───

func tcpPingFallback(proxyId, src string, timeout time.Duration, log *logger.Logger) TestResult {
	endpoint, err := proxyEndpoint(src)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Error: fmt.Sprintf("无法解析代理地址: %v", err)}
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", endpoint, timeout)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency, Error: fmt.Sprintf("TCP 连接失败: %v", err)}
	}
	conn.Close()
	return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: latency}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
