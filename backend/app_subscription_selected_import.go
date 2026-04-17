package backend

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"gopkg.in/yaml.v3"
)

type subscriptionImportStats struct {
	CatalogTotal         int `json:"catalogTotal"`
	ImportedCount        int `json:"importedCount"`
	MissingSelectedCount int `json:"missingSelectedCount"`
}

func parseSelectedNodeKeySet(raw string) map[string]struct{} {
	var list []string
	result := make(map[string]struct{})
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return result
	}
	_ = json.Unmarshal([]byte(raw), &list)
	for _, item := range list {
		if key := strings.TrimSpace(item); key != "" {
			result[key] = struct{}{}
		}
	}
	return result
}

func marshalSelectedNodeKeys(selected map[string]struct{}) string {
	if len(selected) == 0 {
		return ""
	}
	list := make([]string, 0, len(selected))
	for key := range selected {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			list = append(list, trimmed)
		}
	}
	sort.Strings(list)
	data, err := json.Marshal(list)
	if err != nil {
		return ""
	}
	return string(data)
}

func buildSubscriptionCatalog(nodes []config.BrowserProxy) []browser.SubscriptionNode {
	catalog := make([]browser.SubscriptionNode, 0, len(nodes))
	for _, node := range nodes {
		protocol, server, port := extractSubscriptionNodeMeta(node)
		catalog = append(catalog, browser.SubscriptionNode{
			NodeKey:       strings.TrimSpace(node.ProxyId),
			SourceID:      strings.TrimSpace(node.SourceID),
			NodeName:      firstNonEmpty(node.SourceNodeName, node.ProxyName),
			Protocol:      protocol,
			Server:        server,
			Port:          port,
			DisplayGroup:  firstNonEmpty(node.DisplayGroup, node.GroupName),
			ChainMode:     strings.TrimSpace(node.ChainMode),
			UpstreamAlias: strings.TrimSpace(node.UpstreamAlias),
			NodeJSON:      strings.TrimSpace(node.RawProxyConfig),
		})
	}
	return catalog
}

func materializeSelectedNodes(nodes []config.BrowserProxy, selected map[string]struct{}) []config.BrowserProxy {
	if len(selected) == 0 {
		return nil
	}
	byID := make(map[string]config.BrowserProxy, len(nodes))
	included := make(map[string]struct{}, len(selected))
	for _, node := range nodes {
		byID[strings.TrimSpace(node.ProxyId)] = node
	}
	var mark func(string)
	mark = func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := included[id]; ok {
			return
		}
		node, ok := byID[id]
		if !ok {
			return
		}
		included[id] = struct{}{}
		mark(node.UpstreamProxyId)
	}
	for key := range selected {
		mark(key)
	}
	out := make([]config.BrowserProxy, 0, len(included))
	for _, node := range nodes {
		if _, ok := included[strings.TrimSpace(node.ProxyId)]; ok {
			out = append(out, node)
		}
	}
	return out
}

func marshalImportStatsJSON(catalog []browser.SubscriptionNode, selected map[string]struct{}, imported []config.BrowserProxy) string {
	catalogSet := make(map[string]struct{}, len(catalog))
	for _, item := range catalog {
		catalogSet[item.NodeKey] = struct{}{}
	}
	missing := 0
	for key := range selected {
		if _, ok := catalogSet[key]; !ok {
			missing++
		}
	}
	data, err := json.Marshal(subscriptionImportStats{CatalogTotal: len(catalog), ImportedCount: len(imported), MissingSelectedCount: missing})
	if err != nil {
		return ""
	}
	return string(data)
}

func extractSubscriptionNodeMeta(node config.BrowserProxy) (string, string, int) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(node.RawProxyConfig)), &raw); err == nil {
		return strings.TrimSpace(fmt.Sprint(raw["type"])), strings.TrimSpace(fmt.Sprint(raw["server"])), toInt(raw["port"])
	}
	return "", "", 0
}

func toInt(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(v))
		return n
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
