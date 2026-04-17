package backend

import (
	"encoding/json"
	"strings"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/proxy"
)

func mergeSubscriptionSource(input browser.SubscriptionSource, existing *browser.SubscriptionSource) browser.SubscriptionSource {
	if existing == nil {
		return input
	}
	if strings.TrimSpace(input.LastRefreshAt) == "" {
		input.LastRefreshAt = existing.LastRefreshAt
	}
	if strings.TrimSpace(input.LastRefreshStatus) == "" {
		input.LastRefreshStatus = existing.LastRefreshStatus
	}
	if strings.TrimSpace(input.LastError) == "" {
		input.LastError = existing.LastError
	}
	if strings.TrimSpace(input.TrafficUsed) == "" {
		input.TrafficUsed = existing.TrafficUsed
	}
	if strings.TrimSpace(input.TrafficTotal) == "" {
		input.TrafficTotal = existing.TrafficTotal
	}
	if strings.TrimSpace(input.ExpireAt) == "" {
		input.ExpireAt = existing.ExpireAt
	}
	if strings.TrimSpace(input.RawContentHash) == "" {
		input.RawContentHash = existing.RawContentHash
	}
	if strings.TrimSpace(input.ProxyGroupsJSON) == "" {
		input.ProxyGroupsJSON = existing.ProxyGroupsJSON
	}
	if strings.TrimSpace(input.SelectedProxyGroupsJSON) == "" {
		input.SelectedProxyGroupsJSON = existing.SelectedProxyGroupsJSON
	}
	if strings.TrimSpace(input.ImportMode) == "" {
		input.ImportMode = existing.ImportMode
	}
	if strings.TrimSpace(input.SelectedNodeKeysJSON) == "" {
		input.SelectedNodeKeysJSON = existing.SelectedNodeKeysJSON
	}
	if strings.TrimSpace(input.ImportStatsJSON) == "" {
		input.ImportStatsJSON = existing.ImportStatsJSON
	}
	return input
}

func parseSelectedProxyGroups(raw string) map[string]string {
	result := make(map[string]string)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return result
	}
	_ = json.Unmarshal([]byte(raw), &result)
	for key, value := range result {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		delete(result, key)
		if trimmedKey != "" && trimmedValue != "" {
			result[trimmedKey] = trimmedValue
		}
	}
	return result
}

func marshalProxyGroups(groups []proxy.SubscriptionProxyGroup) string {
	if len(groups) == 0 {
		return ""
	}
	data, err := json.Marshal(groups)
	if err != nil {
		return ""
	}
	return string(data)
}

func parseProxyGroups(raw string) []proxy.SubscriptionProxyGroup {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var groups []proxy.SubscriptionProxyGroup
	_ = json.Unmarshal([]byte(raw), &groups)
	return groups
}

func applySelectorChoices(nodes []config.BrowserProxy, groups []proxy.SubscriptionProxyGroup, selected map[string]string) []config.BrowserProxy {
	if len(nodes) == 0 {
		return nodes
	}
	nameToProxyID := make(map[string]string, len(nodes))
	groupMembers := make(map[string][]string, len(groups))
	for _, group := range groups {
		if strings.TrimSpace(group.Name) == "" {
			continue
		}
		groupMembers[group.Name] = append([]string{}, group.Proxies...)
	}
	for _, node := range nodes {
		name := strings.TrimSpace(node.SourceNodeName)
		if name == "" {
			name = strings.TrimSpace(node.ProxyName)
		}
		if name != "" {
			nameToProxyID[name] = node.ProxyId
		}
	}
	out := make([]config.BrowserProxy, 0, len(nodes))
	for _, node := range nodes {
		updated := node
		updated.UpstreamProxyId = ""
		alias := strings.TrimSpace(updated.UpstreamAlias)
		if alias == "" || strings.EqualFold(strings.TrimSpace(updated.ChainMode), "single") {
			updated.ChainStatus = "resolved"
			out = append(out, updated)
			continue
		}
		if proxyID := strings.TrimSpace(nameToProxyID[alias]); proxyID != "" {
			updated.UpstreamProxyId = proxyID
			updated.ChainStatus = "resolved"
			out = append(out, updated)
			continue
		}
		selectedName := strings.TrimSpace(selected[alias])
		if selectedName == "" {
			members := groupMembers[alias]
			if len(members) == 1 {
				selectedName = strings.TrimSpace(members[0])
			}
		}
		if proxyID := strings.TrimSpace(nameToProxyID[selectedName]); proxyID != "" {
			updated.UpstreamProxyId = proxyID
			updated.ChainStatus = "resolved"
		} else if _, ok := groupMembers[alias]; ok {
			updated.ChainStatus = "selector_required"
		} else {
			updated.ChainStatus = "unresolved"
		}
		out = append(out, updated)
	}
	return out
}

func (a *App) syncSubscriptionSelections(source browser.SubscriptionSource) error {
	if a.browserMgr == nil || a.browserMgr.ProxyDAO == nil {
		return nil
	}
	list, err := a.browserMgr.ProxyDAO.List()
	if err != nil {
		return err
	}
	groups := parseProxyGroups(source.ProxyGroupsJSON)
	selected := normalizeSelectedProxyGroups(groups, parseSelectedProxyGroups(source.SelectedProxyGroupsJSON), nil)
	effectiveSelected := selected
	var nodes []config.BrowserProxy
	for _, item := range list {
		if strings.EqualFold(strings.TrimSpace(item.SourceID), strings.TrimSpace(source.SourceID)) {
			nodes = append(nodes, item)
		}
	}
	nodes, err = a.loadSubscriptionSyncNodes(source, nodes)
	if err != nil {
		return err
	}
	selected = normalizeSelectedProxyGroups(groups, selected, nodes)
	effectiveSelected = buildEffectiveSelectedProxyGroups(groups, selected, nodes)
	marshaledSelected := marshalSelectedProxyGroups(selected)
	if marshaledSelected != strings.TrimSpace(source.SelectedProxyGroupsJSON) {
		source.SelectedProxyGroupsJSON = marshaledSelected
		if a.browserMgr.SubscriptionDAO != nil {
			if err := a.browserMgr.SubscriptionDAO.Upsert(source); err != nil {
				return err
			}
		}
	}
	resolved := applySelectorChoices(nodes, groups, effectiveSelected)
	if strings.EqualFold(strings.TrimSpace(source.ImportMode), "selected") {
		resolved = materializeSelectedNodes(resolved, parseSelectedNodeKeySet(source.SelectedNodeKeysJSON))
	}
	if err := a.browserMgr.ProxyDAO.DeleteBySource(source.SourceID); err != nil {
		return err
	}
	for _, node := range resolved {
		if err := a.browserMgr.ProxyDAO.Upsert(node); err != nil {
			return err
		}
	}
	return nil
}
