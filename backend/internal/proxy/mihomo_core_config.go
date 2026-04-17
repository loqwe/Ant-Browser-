package proxy

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func buildExternalBridgeConfig(src string, port int) ([]byte, error) {
	node, entryName, err := buildExternalBridgeNode(src, "proxy-out")
	if err != nil {
		return nil, err
	}
	return marshalExternalBridgeConfig([]map[string]any{node}, entryName, port)
}

func buildExternalChainBridgeConfig(chain ResolvedChain, port int) ([]byte, error) {
	if len(chain.Hops) == 0 {
		return nil, fmt.Errorf("empty mihomo chain")
	}
	nodes := make([]map[string]any, 0, len(chain.Hops))
	names := make([]string, 0, len(chain.Hops))
	used := map[string]int{}
	for index, hop := range chain.Hops {
		node, name, err := buildExternalBridgeNode(chainHopConfig(hop), fmt.Sprintf("proxy-hop-%d", index))
		if err != nil {
			return nil, err
		}
		if used[name] > 0 {
			name = fmt.Sprintf("%s-%d", name, used[name]+1)
			node["name"] = name
		}
		used[name]++
		nodes = append(nodes, node)
		names = append(names, name)
	}
	for index := range nodes {
		if index < len(nodes)-1 {
			nodes[index]["dialer-proxy"] = names[index+1]
		} else {
			delete(nodes[index], "dialer-proxy")
		}
	}
	return marshalExternalBridgeConfig(nodes, names[0], port)
}

func buildExternalBridgeNode(src string, fallbackName string) (map[string]any, string, error) {
	node, err := proxyConfigToMapping(src)
	if err != nil {
		return nil, "", err
	}
	name := strings.TrimSpace(fmt.Sprint(node["name"]))
	if name == "" || name == "<nil>" {
		name = fallbackName
	}
	node["name"] = name
	return node, name, nil
}

func marshalExternalBridgeConfig(nodes []map[string]any, entryName string, port int) ([]byte, error) {
	cfg := map[string]any{
		"socks-port":   port,
		"allow-lan":    false,
		"bind-address": "127.0.0.1",
		"mode":         "global",
		"log-level":    "info",
		"ipv6":         false,
		"profile": map[string]any{
			"store-selected": false,
			"store-fake-ip":  false,
		},
		"dns": map[string]any{
			"enable":           true,
			"ipv6":             false,
			"use-system-hosts": false,
			"enhanced-mode":    "redir-host",
			"nameserver":       []string{"system"},
			"fallback-filter": map[string]any{
				"geoip": false,
			},
		},
		"proxies": nodes,
		"proxy-groups": []map[string]any{{
			"name":    "GLOBAL",
			"type":    "select",
			"proxies": []string{entryName},
		}},
		"rules": []string{"MATCH,GLOBAL"},
	}
	return yaml.Marshal(cfg)
}

func (m *MihomoManager) externalWorkdir(key string) string {
	root := m.AppRoot
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	return filepath.Join(root, "_mihomo_core", key)
}
