package backend

import (
	"encoding/json"
	"strings"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/proxy"
)

type selectorScore struct {
	Priority int
	Latency  int64
	Hit      bool
}

func mergeProxyRuntimeState(nodes, existing []config.BrowserProxy) []config.BrowserProxy {
	byID := make(map[string]config.BrowserProxy, len(existing))
	for _, item := range existing {
		byID[item.ProxyId] = item
	}
	merged := make([]config.BrowserProxy, 0, len(nodes))
	for _, item := range nodes {
		if prev, ok := byID[item.ProxyId]; ok {
			item.LastLatencyMs = prev.LastLatencyMs
			item.LastTestOk = prev.LastTestOk
			item.LastTestedAt = prev.LastTestedAt
			item.LastIPHealthJSON = prev.LastIPHealthJSON
			if strings.EqualFold(strings.TrimSpace(prev.UpstreamAlias), strings.TrimSpace(item.UpstreamAlias)) {
				item.UpstreamProxyId = prev.UpstreamProxyId
			}
		}
		merged = append(merged, item)
	}
	return merged
}

func normalizeSelectedProxyGroups(groups []proxy.SubscriptionProxyGroup, selected map[string]string, nodes []config.BrowserProxy) map[string]string {
	return selectorChoices(groups, selected, nodes, false)
}

func buildEffectiveSelectedProxyGroups(groups []proxy.SubscriptionProxyGroup, selected map[string]string, nodes []config.BrowserProxy) map[string]string {
	return selectorChoices(groups, selected, nodes, true)
}

func selectorChoices(groups []proxy.SubscriptionProxyGroup, selected map[string]string, nodes []config.BrowserProxy, includeFallback bool) map[string]string {
	choices := make(map[string]string, len(groups))
	for _, group := range groups {
		first := ""
		members := make(map[string]struct{}, len(group.Proxies))
		for _, member := range group.Proxies {
			member = strings.TrimSpace(member)
			if member == "" {
				continue
			}
			if first == "" {
				first = member
			}
			members[member] = struct{}{}
		}
		if len(members) == 0 {
			continue
		}
		if choice := strings.TrimSpace(selected[group.Name]); choice != "" {
			if _, ok := members[choice]; ok {
				choices[group.Name] = choice
				continue
			}
		}
		if len(members) == 1 {
			choices[group.Name] = first
			continue
		}
		if strings.EqualFold(strings.TrimSpace(group.Type), "select") {
			if choice := recommendSelectorChoice(group, nodes); choice != "" {
				choices[group.Name] = choice
				continue
			}
			if includeFallback && first != "" {
				choices[group.Name] = first
			}
		}
	}
	return choices
}

func recommendSelectorChoice(group proxy.SubscriptionProxyGroup, nodes []config.BrowserProxy) string {
	groupName := strings.TrimSpace(group.Name)
	memberByID := make(map[string]config.BrowserProxy, len(nodes))
	memberScores := make(map[string]selectorScore, len(group.Proxies))
	for _, item := range nodes {
		memberByID[item.ProxyId] = item
		name := strings.TrimSpace(item.SourceNodeName)
		if name == "" {
			name = strings.TrimSpace(item.ProxyName)
		}
		if name == "" {
			continue
		}
		score := buildSelectorScore(item)
		if _, ok := memberScores[name]; !ok || isBetterSelectorScore(score, memberScores[name]) {
			memberScores[name] = score
		}
	}
	for _, item := range nodes {
		if !strings.EqualFold(strings.TrimSpace(item.UpstreamAlias), groupName) {
			continue
		}
		member, ok := memberByID[strings.TrimSpace(item.UpstreamProxyId)]
		if !ok {
			continue
		}
		name := strings.TrimSpace(member.SourceNodeName)
		if name == "" {
			name = strings.TrimSpace(member.ProxyName)
		}
		if name == "" {
			continue
		}
		score := buildSelectorScore(item)
		if isBetterSelectorScore(score, memberScores[name]) {
			memberScores[name] = score
		}
	}
	bestName := ""
	var bestScore selectorScore
	for _, member := range group.Proxies {
		member = strings.TrimSpace(member)
		score := memberScores[member]
		if !score.Hit {
			continue
		}
		if bestName == "" || isBetterSelectorScore(score, bestScore) {
			bestName = member
			bestScore = score
		}
	}
	return bestName
}

func buildSelectorScore(item config.BrowserProxy) selectorScore {
	if item.LastTestOk {
		return selectorScore{Priority: 2, Latency: item.LastLatencyMs, Hit: true}
	}
	if proxyHealthOK(item.LastIPHealthJSON) {
		return selectorScore{Priority: 1, Hit: true}
	}
	return selectorScore{}
}

func isBetterSelectorScore(left, right selectorScore) bool {
	if !left.Hit {
		return false
	}
	if !right.Hit {
		return true
	}
	if left.Priority != right.Priority {
		return left.Priority > right.Priority
	}
	if left.Priority == 2 {
		if left.Latency <= 0 {
			left.Latency = 1<<62 - 1
		}
		if right.Latency <= 0 {
			right.Latency = 1<<62 - 1
		}
		return left.Latency < right.Latency
	}
	return false
}

func marshalSelectedProxyGroups(selected map[string]string) string {
	if len(selected) == 0 {
		return ""
	}
	data, err := json.Marshal(selected)
	if err != nil {
		return ""
	}
	return string(data)
}

func proxyHealthOK(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	var payload struct {
		Ok bool `json:"ok"`
	}
	return json.Unmarshal([]byte(raw), &payload) == nil && payload.Ok
}
