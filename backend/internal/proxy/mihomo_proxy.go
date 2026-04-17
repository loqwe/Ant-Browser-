package proxy

import (
	"fmt"
	"strings"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/component/proxydialer"
	C "github.com/metacubex/mihomo/constant"
)

type mihomoChainProxy struct {
	C.Proxy
	closers []C.ProxyAdapter
}

func (p *mihomoChainProxy) Close() error {
	var firstErr error
	for _, closer := range p.closers {
		if closer == nil {
			continue
		}
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func SupportsMihomoBridge(src string) bool {
	src = strings.TrimSpace(src)
	if src == "" || strings.EqualFold(src, "direct://") {
		return false
	}
	if isStandardProxyURL(src) {
		return false
	}
	proxyInstance, err := buildMihomoProxy(src, nil)
	if err != nil {
		return false
	}
	_ = proxyInstance.Close()
	return true
}

func SupportsMihomoChain(chain ResolvedChain) bool {
	proxyInstance, err := buildMihomoChainProxy(chain)
	if err != nil {
		return false
	}
	_ = proxyInstance.Close()
	return true
}

func buildMihomoChainProxy(chain ResolvedChain) (C.Proxy, error) {
	if len(chain.Hops) == 0 {
		return nil, fmt.Errorf("empty mihomo chain")
	}
	closers := make([]C.ProxyAdapter, 0, len(chain.Hops))
	var upstream C.Proxy
	for index := len(chain.Hops) - 1; index >= 0; index-- {
		var dialer C.Dialer
		if upstream != nil {
			dialer = proxydialer.New(upstream, true)
		}
		proxyInstance, err := buildMihomoProxy(chainHopConfig(chain.Hops[index]), dialer)
		if err != nil {
			for _, closer := range closers {
				_ = closer.Close()
			}
			return nil, err
		}
		closers = append(closers, proxyInstance)
		upstream = proxyInstance
	}
	return &mihomoChainProxy{Proxy: upstream, closers: closers}, nil
}

func buildMihomoProxy(src string, dialer C.Dialer) (C.Proxy, error) {
	mapping, err := proxyConfigToMapping(src)
	if err != nil {
		return nil, err
	}
	options := make([]adapter.ProxyOption, 0, 1)
	if dialer != nil {
		options = append(options, adapter.WithDialerForAPI(dialer))
	}
	return adapter.ParseProxy(mapping, options...)
}

func isStandardProxyURL(src string) bool {
	l := strings.ToLower(strings.TrimSpace(src))
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "socks5://")
}
