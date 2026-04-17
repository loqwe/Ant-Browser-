package proxy

import (
	"fmt"
	"strings"

	"ant-chrome/backend/internal/config"
)

type ResolvedChain struct {
	RootProxyID string
	Status      string
	Hops        []config.BrowserProxy
}

type ChainResolver struct {
	proxies         []config.BrowserProxy
	byID            map[string]config.BrowserProxy
	bySourceAndName map[string]config.BrowserProxy
}

func NewChainResolver(proxies []config.BrowserProxy) *ChainResolver {
	resolver := &ChainResolver{
		proxies:         append([]config.BrowserProxy{}, proxies...),
		byID:            make(map[string]config.BrowserProxy, len(proxies)),
		bySourceAndName: make(map[string]config.BrowserProxy, len(proxies)),
	}
	for _, item := range proxies {
		resolver.byID[item.ProxyId] = item
		if strings.TrimSpace(item.SourceID) == "" {
			continue
		}
		if name := normalizeAlias(item.SourceNodeName); name != "" {
			resolver.bySourceAndName[item.SourceID+"::"+name] = item
		}
	}
	return resolver
}

func (r *ChainResolver) ResolveProxyChain(proxyID string) (ResolvedChain, error) {
	current, ok := r.byID[proxyID]
	if !ok {
		return ResolvedChain{RootProxyID: proxyID, Status: "broken"}, fmt.Errorf("proxy not found: %s", proxyID)
	}

	visited := make(map[string]struct{}, 4)
	hops := make([]config.BrowserProxy, 0, 4)
	for {
		if _, exists := visited[current.ProxyId]; exists {
			return ResolvedChain{}, fmt.Errorf("proxy chain cycle: %s", current.ProxyId)
		}
		visited[current.ProxyId] = struct{}{}
		hops = append(hops, current)

		alias := strings.TrimSpace(current.UpstreamAlias)
		if alias == "" || strings.EqualFold(strings.TrimSpace(current.ChainMode), "single") {
			break
		}
		next, ok := r.matchUpstream(current.SourceID, alias, current.UpstreamProxyId)
		if !ok {
			return ResolvedChain{RootProxyID: proxyID, Status: "broken", Hops: hops}, fmt.Errorf("missing upstream: %s", alias)
		}
		current = next
	}
	return ResolvedChain{RootProxyID: proxyID, Status: "resolved", Hops: hops}, nil
}

func (r *ChainResolver) matchUpstream(sourceID, alias string, explicitProxyID string) (config.BrowserProxy, bool) {
	if explicitProxyID = strings.TrimSpace(explicitProxyID); explicitProxyID != "" {
		if hit, ok := r.byID[explicitProxyID]; ok {
			return hit, true
		}
	}
	aliasKey := normalizeAlias(alias)
	if aliasKey == "" {
		return config.BrowserProxy{}, false
	}
	if hit, ok := r.bySourceAndName[sourceID+"::"+aliasKey]; ok {
		return hit, true
	}
	if hit, ok := r.uniqueSourceMatch(sourceID, func(item config.BrowserProxy) bool {
		return normalizeAlias(item.ProxyName) == aliasKey
	}); ok {
		return hit, true
	}
	if hit, ok := r.uniqueSourceMatch(sourceID, func(item config.BrowserProxy) bool {
		return normalizeAlias(item.DisplayGroup) == aliasKey || normalizeAlias(item.RawProxyGroupName) == aliasKey
	}); ok {
		return hit, true
	}
	return config.BrowserProxy{}, false
}

func (r *ChainResolver) uniqueSourceMatch(sourceID string, match func(config.BrowserProxy) bool) (config.BrowserProxy, bool) {
	var hit config.BrowserProxy
	matched := 0
	for _, item := range r.proxies {
		if sourceID != "" && !strings.EqualFold(strings.TrimSpace(item.SourceID), strings.TrimSpace(sourceID)) {
			continue
		}
		if !match(item) {
			continue
		}
		hit = item
		matched++
		if matched > 1 {
			return config.BrowserProxy{}, false
		}
	}
	return hit, matched == 1
}

func normalizeAlias(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func ResolveRuntimeChain(src string, proxies []config.BrowserProxy, proxyID string) (string, ResolvedChain, bool, error) {
	src = strings.TrimSpace(src)
	if strings.TrimSpace(proxyID) == "" {
		return src, ResolvedChain{}, false, nil
	}
	chain, err := NewChainResolver(proxies).ResolveProxyChain(proxyID)
	if err != nil {
		if src != "" {
			return src, chain, false, nil
		}
		return "", chain, false, err
	}
	if src == "" && len(chain.Hops) > 0 {
		src = chainHopConfig(chain.Hops[0])
	}
	return src, chain, len(chain.Hops) > 1, nil
}

func ChainUsesSingBox(chain ResolvedChain) bool {
	if len(chain.Hops) == 0 {
		return false
	}
	_, err := CompileSingBoxChain(chain)
	return err == nil
}
