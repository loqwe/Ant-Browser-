package proxy

import (
	"ant-chrome/backend/internal/config"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type CompiledXrayChain struct {
	EntryTag   string
	DNSServers string
	Outbounds  []map[string]interface{}
}

type CompiledSingBoxChain struct {
	EntryTag  string
	Outbounds []map[string]interface{}
}

func CompileXrayChain(chain ResolvedChain) (CompiledXrayChain, error) {
	outbounds := make([]map[string]interface{}, 0, len(chain.Hops))
	for index, hop := range chain.Hops {
		outbound, err := buildChainXrayOutbound(hop)
		if err != nil {
			return CompiledXrayChain{}, err
		}
		outbound["tag"] = fmt.Sprintf("proxy-hop-%d", index)
		if index < len(chain.Hops)-1 {
			attachXrayDialerProxy(outbound, fmt.Sprintf("proxy-hop-%d", index+1))
		}
		outbounds = append(outbounds, outbound)
	}
	return CompiledXrayChain{EntryTag: "proxy-hop-0", DNSServers: firstChainDNSServers(chain.Hops), Outbounds: outbounds}, nil
}

func CompileSingBoxChain(chain ResolvedChain) (CompiledSingBoxChain, error) {
	outbounds := make([]map[string]interface{}, 0, len(chain.Hops))
	for index, hop := range chain.Hops {
		outbound, err := buildChainSingBoxOutbound(hop)
		if err != nil {
			return CompiledSingBoxChain{}, err
		}
		outbound["tag"] = fmt.Sprintf("proxy-hop-%d", index)
		if index < len(chain.Hops)-1 {
			outbound["detour"] = fmt.Sprintf("proxy-hop-%d", index+1)
		}
		outbounds = append(outbounds, outbound)
	}
	return CompiledSingBoxChain{EntryTag: "proxy-hop-0", Outbounds: outbounds}, nil
}

func buildChainXrayOutbound(hop config.BrowserProxy) (map[string]interface{}, error) {
	standardProxy, outbound, err := ParseProxyNode(chainHopConfig(hop))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(standardProxy) != "" {
		return buildStandardXrayOutbound(standardProxy)
	}
	if outbound == nil {
		return nil, fmt.Errorf("empty xray outbound")
	}
	return cloneMap(outbound)
}

func buildChainSingBoxOutbound(hop config.BrowserProxy) (map[string]interface{}, error) {
	raw := chainHopConfig(hop)
	lower := strings.ToLower(strings.TrimSpace(raw))
	if strings.HasPrefix(lower, "socks5://") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return buildStandardSingBoxOutbound(raw)
	}
	outbound, err := BuildSingBoxOutbound(raw)
	if err != nil {
		return nil, err
	}
	return cloneMap(outbound)
}

func buildStandardXrayOutbound(proxyURL string) (map[string]interface{}, error) {
	u, err := url.Parse(strings.TrimSpace(proxyURL))
	if err != nil {
		return nil, err
	}
	port, _ := strconv.Atoi(u.Port())
	server := map[string]interface{}{"address": u.Hostname(), "port": port}
	if u.User != nil {
		pass, _ := u.User.Password()
		server["users"] = []map[string]string{{"user": u.User.Username(), "pass": pass}}
	}
	protocol := "http"
	if strings.EqualFold(u.Scheme, "socks5") {
		protocol = "socks"
	}
	return map[string]interface{}{"protocol": protocol, "settings": map[string]interface{}{"servers": []interface{}{server}}}, nil
}

func buildStandardSingBoxOutbound(proxyURL string) (map[string]interface{}, error) {
	u, err := url.Parse(strings.TrimSpace(proxyURL))
	if err != nil {
		return nil, err
	}
	port, _ := strconv.Atoi(u.Port())
	outbound := map[string]interface{}{"type": strings.ToLower(u.Scheme), "server": u.Hostname(), "server_port": port}
	if strings.EqualFold(u.Scheme, "socks5") {
		outbound["type"] = "socks"
	}
	if u.User != nil {
		pass, _ := u.User.Password()
		outbound["username"] = u.User.Username()
		outbound["password"] = pass
	}
	return outbound, nil
}

func chainHopConfig(hop config.BrowserProxy) string {
	if strings.TrimSpace(hop.ProxyConfig) != "" {
		return strings.TrimSpace(hop.ProxyConfig)
	}
	return strings.TrimSpace(hop.RawProxyConfig)
}

func firstChainDNSServers(hops []config.BrowserProxy) string {
	for _, hop := range hops {
		if strings.TrimSpace(hop.DnsServers) != "" {
			return strings.TrimSpace(hop.DnsServers)
		}
	}
	return ""
}

func cloneMap(input map[string]interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var output map[string]interface{}
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, err
	}
	return output, nil
}

func attachXrayDialerProxy(outbound map[string]interface{}, nextTag string) {
	streamSettings, _ := outbound["streamSettings"].(map[string]interface{})
	if streamSettings == nil {
		streamSettings = make(map[string]interface{})
	}
	sockopt, _ := streamSettings["sockopt"].(map[string]interface{})
	if sockopt == nil {
		sockopt = make(map[string]interface{})
	}
	sockopt["dialerProxy"] = nextTag
	streamSettings["sockopt"] = sockopt
	outbound["streamSettings"] = streamSettings
	delete(outbound, "proxySettings")
}
