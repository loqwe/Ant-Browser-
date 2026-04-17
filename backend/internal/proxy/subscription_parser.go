package proxy

import (
	"crypto/sha1"
	"fmt"
	"strings"

	"ant-chrome/backend/internal/config"
	"gopkg.in/yaml.v3"
)

type SubscriptionProxyGroup struct {
	Name    string   `json:"name"`
	Type    string   `json:"type,omitempty"`
	Proxies []string `json:"proxies,omitempty"`
}

type SubscriptionDocument struct {
	SourceID  string
	SourceURL string
	Nodes     []config.BrowserProxy
	Groups    []SubscriptionProxyGroup
}

func ParseSubscriptionDocument(raw []byte, sourceID, sourceURL string) (SubscriptionDocument, error) {
	var payload interface{}
	if err := yaml.Unmarshal(raw, &payload); err != nil {
		return SubscriptionDocument{}, fmt.Errorf("parse subscription yaml failed: %w", err)
	}
	root := toStringMap(payload)
	if root == nil {
		return SubscriptionDocument{}, fmt.Errorf("invalid subscription payload")
	}
	proxies, _ := root["proxies"].([]interface{})
	groupByNode, groups := buildSubscriptionGroups(root)
	nameSet := make(map[string]struct{}, len(proxies))
	for _, item := range proxies {
		node := toStringMap(item)
		name := strings.TrimSpace(getMapString(node, "name"))
		if name != "" {
			nameSet[name] = struct{}{}
		}
	}
	result := SubscriptionDocument{SourceID: sourceID, SourceURL: sourceURL, Groups: groups}
	for index, item := range proxies {
		node := toStringMap(item)
		name := strings.TrimSpace(getMapString(node, "name"))
		if name == "" {
			continue
		}
		rawNodeBytes, err := yaml.Marshal(node)
		if err != nil {
			return SubscriptionDocument{}, fmt.Errorf("marshal proxy node failed: %w", err)
		}
		rawNode := strings.TrimSpace(string(rawNodeBytes))
		proxyConfig := rawNode
		upstreamAlias := strings.TrimSpace(getMapString(node, "dialer-proxy"))
		chainMode := "single"
		chainStatus := "resolved"
		if upstreamAlias != "" {
			chainMode = "chained"
			if _, ok := nameSet[upstreamAlias]; !ok {
				chainStatus = "selector_required"
			}
		}
		displayGroup := strings.TrimSpace(groupByNode[name])
		result.Nodes = append(result.Nodes, config.BrowserProxy{
			ProxyId:           subscriptionProxyID(sourceID, name),
			ProxyName:         name,
			ProxyConfig:       proxyConfig,
			GroupName:         displayGroup,
			SortOrder:         index,
			SourceID:          sourceID,
			SourceURL:         sourceURL,
			SourceNodeName:    name,
			DisplayGroup:      displayGroup,
			ChainMode:         chainMode,
			UpstreamAlias:     upstreamAlias,
			RawProxyGroupName: displayGroup,
			RawProxyConfig:    rawNode,
			ChainStatus:       chainStatus,
		})
	}
	return result, nil
}

func buildSubscriptionGroups(root map[string]interface{}) (map[string]string, []SubscriptionProxyGroup) {
	result := make(map[string]string)
	groups := make([]SubscriptionProxyGroup, 0)
	items, _ := root["proxy-groups"].([]interface{})
	for _, item := range items {
		group := toStringMap(item)
		name := strings.TrimSpace(getMapString(group, "name"))
		if name == "" {
			continue
		}
		members, _ := group["proxies"].([]interface{})
		memberNames := make([]string, 0, len(members))
		for _, member := range members {
			memberName := strings.TrimSpace(fmt.Sprint(member))
			if memberName == "" {
				continue
			}
			memberNames = append(memberNames, memberName)
			if result[memberName] == "" {
				result[memberName] = name
			}
		}
		groups = append(groups, SubscriptionProxyGroup{Name: name, Type: strings.TrimSpace(getMapString(group, "type")), Proxies: memberNames})
	}
	return result, groups
}

func subscriptionProxyID(sourceID, nodeName string) string {
	sum := sha1.Sum([]byte(sourceID + "::" + nodeName))
	return fmt.Sprintf("sub-%x", sum[:8])
}
