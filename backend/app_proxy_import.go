package backend

import (
	"ant-chrome/backend/internal/logger"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	maxClashSubscriptionBytes = 8 * 1024 * 1024
	clashSubscriptionTimeout  = 45 * time.Second
)

// BrowserProxyFetchClashByURL 拉取 Clash 订阅 URL，并返回可直接导入的 YAML 文本与建议配置。
func (a *App) BrowserProxyFetchClashByURL(rawURL string) (map[string]interface{}, error) {
	log := logger.New("Subscription")
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("订阅 URL 不能为空")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Host == "" {
		return nil, fmt.Errorf("URL 格式无效")
	}
	scheme := strings.ToLower(strings.TrimSpace(parsedURL.Scheme))
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("仅支持 http/https URL")
	}

	log.Info("开始拉取订阅", logger.F("url", parsedURL.String()), logger.F("timeout_ms", clashSubscriptionTimeout.Milliseconds()))

	if goruntime.GOOS == "windows" {
		if body, err := downloadSubscriptionBytesWithCurl(parsedURL.String()); err == nil {
			log.Info("curl 拉取订阅成功", logger.F("url", parsedURL.String()), logger.F("size", len(body)))
			return buildClashSubscriptionPreview(parsedURL, body, log)
		} else {
			log.Warn("curl 拉取订阅失败，准备回退 PowerShell", logger.F("url", parsedURL.String()), logger.F("error", err.Error()))
		}

		if body, err := downloadSubscriptionBytesWithPowerShell(parsedURL.String()); err == nil {
			log.Info("PowerShell 拉取订阅成功", logger.F("url", parsedURL.String()), logger.F("size", len(body)))
			return buildClashSubscriptionPreview(parsedURL, body, log)
		} else {
			log.Warn("PowerShell 拉取订阅失败，准备回退 Go HTTP", logger.F("url", parsedURL.String()), logger.F("error", err.Error()))
		}
	}

	client := &http.Client{Timeout: clashSubscriptionTimeout}
	var body []byte
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequest(http.MethodGet, parsedURL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("创建请求失败: %w", err)
		}
		req.Header.Set("User-Agent", "clash-verge/2.0 ant-chrome/1.0")
		req.Header.Set("Accept", "application/yaml,text/yaml,text/plain,*/*")
		req.Header.Set("Cache-Control", "no-cache")
		log.Info("订阅拉取尝试", logger.F("url", parsedURL.String()), logger.F("attempt", attempt))

		resp, err := client.Do(req)
		if err != nil {
			if attempt < 3 {
				log.Warn("订阅拉取失败，准备重试", logger.F("url", parsedURL.String()), logger.F("attempt", attempt), logger.F("error", err.Error()))
				time.Sleep(time.Duration(attempt) * 800 * time.Millisecond)
				continue
			}
			log.Error("订阅拉取失败", logger.F("url", parsedURL.String()), logger.F("attempt", attempt), logger.F("error", err.Error()))
			return nil, fmt.Errorf("拉取订阅失败: %w", err)
		}

		func() {
			defer resp.Body.Close()
			log.Info("订阅响应已返回", logger.F("url", parsedURL.String()), logger.F("attempt", attempt), logger.F("status", resp.StatusCode))
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				err = fmt.Errorf("拉取订阅失败: HTTP %d", resp.StatusCode)
				return
			}
			body, err = io.ReadAll(io.LimitReader(resp.Body, maxClashSubscriptionBytes+1))
			if err != nil {
				err = fmt.Errorf("读取订阅内容失败: %w", err)
				return
			}
			if len(body) > maxClashSubscriptionBytes {
				err = fmt.Errorf("订阅内容过大（超过 8MB）")
				return
			}
			err = nil
		}()

		if err == nil {
			break
		}
		if attempt < 3 && shouldRetrySubscriptionFetch(err) {
			log.Warn("订阅拉取结果异常，准备重试", logger.F("url", parsedURL.String()), logger.F("attempt", attempt), logger.F("error", err.Error()))
			time.Sleep(time.Duration(attempt) * 800 * time.Millisecond)
			continue
		}
		log.Error("订阅拉取结果异常", logger.F("url", parsedURL.String()), logger.F("attempt", attempt), logger.F("error", err.Error()))
		return nil, err
	}

	return buildClashSubscriptionPreview(parsedURL, body, log)
}

func buildClashSubscriptionPreview(parsedURL *url.URL, body []byte, log *logger.Logger) (map[string]interface{}, error) {
	content, payload, err := normalizeClashSubscriptionContent(body)
	if err != nil {
		if log != nil {
			log.Error("订阅解析失败", logger.F("url", parsedURL.String()), logger.F("error", err.Error()))
		}
		return nil, err
	}

	proxyCount := clashProxyCount(payload)
	if proxyCount <= 0 {
		err = fmt.Errorf("未检测到可导入的 proxies 节点")
		if log != nil {
			log.Error("订阅解析失败", logger.F("url", parsedURL.String()), logger.F("error", err.Error()))
		}
		return nil, err
	}

	dnsYAML := extractClashDNSYAML(payload)
	suggestedGroup := suggestClashGroupName(payload, parsedURL.Hostname())
	if log != nil {
		log.Info("订阅拉取成功", logger.F("url", parsedURL.String()), logger.F("proxy_count", proxyCount))
	}

	return map[string]interface{}{
		"url":            parsedURL.String(),
		"content":        content,
		"proxyCount":     proxyCount,
		"dnsServers":     dnsYAML,
		"suggestedGroup": suggestedGroup,
	}, nil
}

func downloadSubscriptionBytesWithCurl(targetURL string) ([]byte, error) {
	curlPath, err := exec.LookPath("curl.exe")
	if err != nil {
		return nil, err
	}
	tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("antbrowser-sub-%d.yaml", time.Now().UnixNano()))
	defer os.Remove(tempFile)
	cmd := exec.Command(curlPath, "--http1.1", "-L", "--fail", "-A", "clash-verge/2.0 ant-chrome/1.0", "-H", "Accept: application/yaml,text/yaml,text/plain,*/*", "-H", "Cache-Control: no-cache", "-o", tempFile, targetURL)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%v %s", err, strings.TrimSpace(string(output)))
	}
	data, err := os.ReadFile(tempFile)
	if err != nil {
		return nil, err
	}
	if len(data) > maxClashSubscriptionBytes {
		return nil, fmt.Errorf("订阅内容过大（超过 8MB）")
	}
	return data, nil
}

func downloadSubscriptionBytesWithPowerShell(targetURL string) ([]byte, error) {
	tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("antbrowser-sub-%d.yaml", time.Now().UnixNano()))
	defer os.Remove(tempFile)
	cmd := exec.Command("powershell.exe", "-Command", "$ProgressPreference='SilentlyContinue'; Invoke-WebRequest -UseBasicParsing -MaximumRedirection 5 -Uri $args[0] -OutFile $args[1] -Headers @{ 'User-Agent'='clash-verge/2.0 ant-chrome/1.0'; 'Accept'='application/yaml,text/yaml,text/plain,*/*'; 'Cache-Control'='no-cache' } | Out-Null", targetURL, tempFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%v %s", err, strings.TrimSpace(string(output)))
	}
	data, err := os.ReadFile(tempFile)
	if err != nil {
		return nil, err
	}
	if len(data) > maxClashSubscriptionBytes {
		return nil, fmt.Errorf("订阅内容过大（超过 8MB）")
	}
	return data, nil
}

func shouldRetrySubscriptionFetch(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "timeout") ||
		strings.Contains(message, "deadline exceeded") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "eof") ||
		strings.Contains(message, "http 408") ||
		strings.Contains(message, "http 425") ||
		strings.Contains(message, "http 429") ||
		strings.Contains(message, "http 500") ||
		strings.Contains(message, "http 502") ||
		strings.Contains(message, "http 503") ||
		strings.Contains(message, "http 504")
}

func normalizeClashSubscriptionContent(body []byte) (string, interface{}, error) {
	baseText := strings.TrimSpace(strings.ReplaceAll(string(body), "\r\n", "\n"))
	if baseText == "" {
		return "", nil, fmt.Errorf("订阅内容为空")
	}

	tryTexts := make([]string, 0, 4)
	tryTexts = append(tryTexts, baseText)

	if unescaped, err := url.QueryUnescape(baseText); err == nil {
		unescaped = strings.TrimSpace(strings.ReplaceAll(unescaped, "\r\n", "\n"))
		if unescaped != "" && unescaped != baseText {
			tryTexts = append(tryTexts, unescaped)
		}
	}

	if decoded, ok := decodeBase64Text(baseText); ok {
		tryTexts = append(tryTexts, decoded)
	}

	for _, text := range tryTexts {
		payload, ok := parseClashPayload(text)
		if !ok {
			continue
		}
		if clashProxyCount(payload) > 0 {
			return text, payload, nil
		}
	}

	return "", nil, fmt.Errorf("URL 内容不是有效 Clash YAML（需包含 proxies）")
}

func decodeBase64Text(raw string) (string, bool) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return "", false
	}
	padded := candidate
	if mod := len(padded) % 4; mod != 0 {
		padded += strings.Repeat("=", 4-mod)
	}

	encoders := []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding}
	for _, enc := range encoders {
		if data, err := enc.DecodeString(candidate); err == nil {
			decoded := strings.TrimSpace(strings.ReplaceAll(string(data), "\r\n", "\n"))
			if decoded != "" {
				return decoded, true
			}
		}
		if data, err := enc.DecodeString(padded); err == nil {
			decoded := strings.TrimSpace(strings.ReplaceAll(string(data), "\r\n", "\n"))
			if decoded != "" {
				return decoded, true
			}
		}
	}
	return "", false
}

func parseClashPayload(text string) (interface{}, bool) {
	var payload interface{}
	if err := yaml.Unmarshal([]byte(text), &payload); err != nil {
		return nil, false
	}
	return payload, true
}

func clashProxyCount(payload interface{}) int {
	if m := toStringMap(payload); m != nil {
		if arr, ok := m["proxies"].([]interface{}); ok {
			return len(arr)
		}
		if arr, ok := m["proxy"].([]interface{}); ok {
			return len(arr)
		}
		if arr, ok := m["Proxy"].([]interface{}); ok {
			return len(arr)
		}
	}
	if arr, ok := payload.([]interface{}); ok {
		return len(arr)
	}
	return 0
}

func extractClashDNSYAML(payload interface{}) string {
	m := toStringMap(payload)
	if m == nil {
		return ""
	}
	dnsRaw, exists := m["dns"]
	if !exists || dnsRaw == nil {
		return ""
	}
	data, err := yaml.Marshal(map[string]interface{}{"dns": dnsRaw})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func suggestClashGroupName(payload interface{}, fallbackHost string) string {
	fallbackHost = strings.TrimSpace(fallbackHost)
	m := toStringMap(payload)
	if m != nil {
		if groups, ok := m["proxy-groups"].([]interface{}); ok {
			for _, item := range groups {
				if groupMap := toStringMap(item); groupMap != nil {
					if name := strings.TrimSpace(getMapString(groupMap, "name")); name != "" {
						return name
					}
				}
			}
		}
	}
	if strings.HasPrefix(strings.ToLower(fallbackHost), "www.") {
		fallbackHost = fallbackHost[4:]
	}
	return fallbackHost
}

func toStringMap(value interface{}) map[string]interface{} {
	switch m := value.(type) {
	case map[string]interface{}:
		return m
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, v := range m {
			out[fmt.Sprint(k)] = v
		}
		return out
	default:
		return nil
	}
}

func getMapString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	value, ok := m[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}
