package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"

	xproxy "golang.org/x/net/proxy"
)

// buildProxyHTTPClient 根据代理配置构建 HTTP 客户端，统一用于测速/健康检测场景。
func buildProxyHTTPClient(
	src string,
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	mihomoMgr *MihomoManager,
	timeout time.Duration,
) (*http.Client, error) {
	src, chain, chained, err := ResolveRuntimeChain(src, proxies, proxyId)
	if err != nil {
		return nil, err
	}
	l := strings.ToLower(strings.TrimSpace(src))
	if chained {
		if SupportsMihomoChain(chain) {
			if mihomoMgr == nil {
				return nil, fmt.Errorf("mihomo manager unavailable")
			}
			socks5Addr, chainErr := mihomoMgr.EnsureChainBridge(chain)
			if chainErr != nil {
				return nil, fmt.Errorf("mihomo chain bridge failed: %w", chainErr)
			}
			return buildSocks5HTTPClient(strings.TrimPrefix(socks5Addr, "socks5://"), timeout)
		}
		if ChainUsesSingBox(chain) {
			if singboxMgr == nil {
				return nil, fmt.Errorf("sing-box ???????")
			}
			socks5Addr, chainErr := singboxMgr.EnsureChainBridge(chain)
			if chainErr != nil {
				return nil, fmt.Errorf("sing-box ????????: %w", chainErr)
			}
			return buildSocks5HTTPClient(strings.TrimPrefix(socks5Addr, "socks5://"), timeout)
		}
		if xrayMgr == nil {
			return nil, fmt.Errorf("xray ???????")
		}
		socks5Addr, chainErr := xrayMgr.EnsureChainBridge(chain)
		if chainErr != nil {
			return nil, fmt.Errorf("xray ????????: %w", chainErr)
		}
		return buildSocks5HTTPClient(strings.TrimPrefix(socks5Addr, "socks5://"), timeout)
	}
	if l == "" || l == "direct://" {
		return &http.Client{Timeout: timeout}, nil
	}
	if SupportsMihomoBridge(src) {
		if mihomoMgr == nil {
			return nil, fmt.Errorf("mihomo manager unavailable")
		}
		socks5Addr, bridgeErr := mihomoMgr.EnsureBridge(src)
		if bridgeErr != nil {
			return nil, fmt.Errorf("mihomo bridge failed: %w", bridgeErr)
		}
		return buildSocks5HTTPClient(strings.TrimPrefix(socks5Addr, "socks5://"), timeout)
	}
	if IsSingBoxProtocol(src) {
		if singboxMgr == nil {
			return nil, fmt.Errorf("sing-box ???????")
		}
		socks5Addr, bridgeErr := singboxMgr.EnsureBridge(src, proxies, proxyId)
		if bridgeErr != nil {
			return nil, fmt.Errorf("sing-box ??????: %w", bridgeErr)
		}
		return buildSocks5HTTPClient(strings.TrimPrefix(socks5Addr, "socks5://"), timeout)
	}
	if RequiresBridge(src, proxies, proxyId) {
		if xrayMgr == nil {
			return nil, fmt.Errorf("xray ???????")
		}
		socks5Addr, bridgeErr := xrayMgr.EnsureBridge(src, proxies, proxyId)
		if bridgeErr != nil {
			return nil, fmt.Errorf("xray ??????: %w", bridgeErr)
		}
		return buildSocks5HTTPClient(strings.TrimPrefix(socks5Addr, "socks5://"), timeout)
	}
	if strings.HasPrefix(l, "socks5://") {
		u, parseErr := url.Parse(src)
		if parseErr != nil {
			return nil, fmt.Errorf("SOCKS5 ??????: %w", parseErr)
		}
		var auth *xproxy.Auth
		if u.User != nil {
			pass, _ := u.User.Password()
			auth = &xproxy.Auth{User: u.User.Username(), Password: pass}
		}
		dialer, dialErr := xproxy.SOCKS5("tcp", u.Host, auth, xproxy.Direct)
		if dialErr != nil {
			return nil, fmt.Errorf("SOCKS5 dialer ????: %w", dialErr)
		}
		contextDialer, ok := dialer.(xproxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("SOCKS5 dialer ??? ContextDialer")
		}
		return &http.Client{Transport: &http.Transport{DialContext: contextDialer.DialContext}, Timeout: timeout}, nil
	}
	proxyURL, parseErr := url.Parse(src)
	if parseErr != nil {
		return nil, fmt.Errorf("????????: %w", parseErr)
	}
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: timeout}, nil
}

func buildSocks5HTTPClient(socks5Host string, timeout time.Duration) (*http.Client, error) {
	dialer, err := xproxy.SOCKS5("tcp", socks5Host, nil, xproxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 dialer 创建失败: %w", err)
	}
	contextDialer, ok := dialer.(xproxy.ContextDialer)
	if !ok {
		return nil, fmt.Errorf("SOCKS5 dialer 不支持 ContextDialer")
	}
	transport := &http.Transport{DialContext: contextDialer.DialContext}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}
