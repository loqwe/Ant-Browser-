package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"
)

const defaultIPPureInfoURL = "https://my.ippure.com/v1/info"

const fallbackIPAPIURL = "https://ipapi.co/json/"

type ipHealthEndpoint struct {
	Source string
	URL    string
}

var defaultIPHealthEndpoints = []ipHealthEndpoint{
	{Source: "ippure", URL: defaultIPPureInfoURL},
	{Source: "ipapi", URL: fallbackIPAPIURL},
}

// FetchIPPureInfo 通过指定代理链路查询 IPPure 的出口 IP 健康信息。
// 返回值为第三方接口原始 JSON（map 形式），不做本地评分计算。
func FetchIPPureInfo(
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	mihomoMgr *MihomoManager,
) (map[string]interface{}, error) {
	src := ""
	for _, item := range proxies {
		if strings.EqualFold(item.ProxyId, proxyId) {
			src = strings.TrimSpace(item.ProxyConfig)
			break
		}
	}
	if src == "" {
		return nil, fmt.Errorf("未找到代理配置")
	}

	client, err := buildIPPureHTTPClient(src, proxyId, proxies, xrayMgr, singboxMgr, mihomoMgr, 20*time.Second)
	if err != nil {
		return nil, err
	}
	return fetchIPHealthInfoWithClient(client, defaultIPHealthEndpoints)
}

func fetchIPHealthInfoWithClient(client *http.Client, endpoints []ipHealthEndpoint) (map[string]interface{}, error) {
	errors := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		req, _ := http.NewRequest(http.MethodGet, endpoint.URL, nil)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "AntChrome/1.0")
		resp, err := client.Do(req)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", endpoint.Source, err))
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", endpoint.Source, readErr))
			continue
		}
		data, parseErr := parseIPHealthPayload(endpoint.Source, resp.StatusCode, body)
		if parseErr != nil {
			errors = append(errors, parseErr.Error())
			continue
		}
		data["_source"] = endpoint.Source
		return data, nil
	}
	return nil, fmt.Errorf("IP 健康查询失败: %s", strings.Join(errors, " | "))
}

func parseIPHealthPayload(source string, statusCode int, body []byte) (map[string]interface{}, error) {
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("%s HTTP %d: %s", source, statusCode, bodySnippet(body, 180))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("%s JSON 解析失败: %w", source, err)
	}
	switch source {
	case "ippure":
		return result, nil
	case "ipapi":
		result["ip"] = firstNonEmpty(healthMapString(result, "ip"), healthMapString(result, "query"))
		result["country"] = firstNonEmpty(healthMapString(result, "country_name"), healthMapString(result, "country"))
		result["countryCode"] = strings.ToUpper(firstNonEmpty(healthMapString(result, "country_code"), healthMapString(result, "countryCode")))
		result["region"] = firstNonEmpty(healthMapString(result, "region"), healthMapString(result, "region_name"))
		result["city"] = healthMapString(result, "city")
		result["asOrganization"] = firstNonEmpty(healthMapString(result, "org"), healthMapString(result, "asn_org"))
		result["timezone"] = healthMapString(result, "timezone")
		if strings.TrimSpace(healthMapString(result, "ip")) == "" {
			return nil, fmt.Errorf("%s 缺少 IP 字段", source)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("不支持的 IP 健康来源: %s", source)
	}
}

func healthMapString(m map[string]interface{}, key string) string {
	value, ok := m[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func buildIPPureHTTPClient(
	src string,
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	mihomoMgr *MihomoManager,
	timeout time.Duration,
) (*http.Client, error) {
	return buildProxyHTTPClient(src, proxyId, proxies, xrayMgr, singboxMgr, mihomoMgr, timeout)
}

func bodySnippet(body []byte, max int) string {
	s := strings.TrimSpace(string(body))
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
