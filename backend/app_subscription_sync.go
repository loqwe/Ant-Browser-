package backend

import (
	"strings"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
)

func (a *App) loadSubscriptionSyncNodes(source browser.SubscriptionSource, existing []config.BrowserProxy) ([]config.BrowserProxy, error) {
	if a.browserMgr == nil || a.browserMgr.SubscriptionNodeDAO == nil {
		return append([]config.BrowserProxy{}, existing...), nil
	}
	catalog, err := a.browserMgr.SubscriptionNodeDAO.ListBySource(source.SourceID)
	if err != nil {
		return nil, err
	}
	if len(catalog) == 0 {
		return append([]config.BrowserProxy{}, existing...), nil
	}
	byID := make(map[string]config.BrowserProxy, len(existing))
	for _, item := range existing {
		byID[strings.TrimSpace(item.ProxyId)] = item
	}
	nodes := make([]config.BrowserProxy, 0, len(catalog))
	for _, item := range catalog {
		key := strings.TrimSpace(item.NodeKey)
		node := byID[key]
		node.ProxyId = key
		node.ProxyName = firstNonEmpty(item.NodeName, node.ProxyName)
		node.ProxyConfig = firstNonEmpty(strings.TrimSpace(item.NodeJSON), node.ProxyConfig)
		node.SourceID = firstNonEmpty(item.SourceID, node.SourceID, source.SourceID)
		node.SourceURL = firstNonEmpty(node.SourceURL, source.URL)
		node.SourceNodeName = firstNonEmpty(item.NodeName, node.SourceNodeName, node.ProxyName)
		node.DisplayGroup = firstNonEmpty(item.DisplayGroup, node.DisplayGroup)
		node.GroupName = firstNonEmpty(node.GroupName, node.DisplayGroup)
		node.ChainMode = firstNonEmpty(item.ChainMode, node.ChainMode, "single")
		node.UpstreamAlias = firstNonEmpty(item.UpstreamAlias, node.UpstreamAlias)
		node.RawProxyConfig = firstNonEmpty(strings.TrimSpace(item.NodeJSON), node.RawProxyConfig, node.ProxyConfig)
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (a *App) reconcileSubscriptionSelections() error {
	if a.browserMgr == nil || a.browserMgr.SubscriptionDAO == nil {
		return nil
	}
	list, err := a.browserMgr.SubscriptionDAO.List()
	if err != nil {
		return err
	}
	for _, item := range list {
		if strings.TrimSpace(item.SourceID) == "" {
			continue
		}
		if err := a.syncSubscriptionSelections(item); err != nil {
			return err
		}
	}
	return nil
}
