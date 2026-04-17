package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func (m *XrayManager) EnsureChainBridge(chain ResolvedChain) (string, error) {
	socksURL, _, err := m.ensureChainBridge(chain, false)
	return socksURL, err
}

func (m *XrayManager) AcquireChainBridge(chain ResolvedChain) (string, string, error) {
	return m.ensureChainBridge(chain, true)
}

func (m *XrayManager) ensureChainBridge(chain ResolvedChain, pin bool) (string, string, error) {
	compiled, err := CompileXrayChain(chain)
	if err != nil {
		return "", "", err
	}
	payload, _ := json.Marshal(compiled.Outbounds)
	key := computeNodeKey(string(payload) + "\x00" + compiled.DNSServers)
	if socksURL, reused := m.tryReuseBridge(key, pin); reused {
		return socksURL, key, nil
	}
	binaryPath, err := m.resolveBinary()
	if err != nil {
		return "", "", err
	}
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		port, portErr := nextAvailablePort()
		if portErr != nil {
			lastErr = portErr
			continue
		}
		cfgPath, cfgErr := m.buildChainRuntimeConfig(key, compiled, port)
		if cfgErr != nil {
			return "", "", cfgErr
		}
		cmd := exec.Command(binaryPath, "run", "-c", cfgPath)
		hideWindow(cmd)
		cmd.Dir = filepath.Dir(cfgPath)
		if err := cmd.Start(); err != nil {
			lastErr = err
			continue
		}
		bridge := &XrayBridge{NodeKey: key, Port: port, Cmd: cmd, Pid: cmd.Process.Pid, Running: true, LastUsedAt: time.Now()}
		if pin {
			bridge.RefCount = 1
		}
		if err := waitPortReady("127.0.0.1", port, 10*time.Second); err != nil {
			bridge.Stopping = true
			m.stopBridgeProcess(bridge)
			lastErr = err
			continue
		}
		if socksURL, reused := m.registerBridge(key, bridge, pin); reused {
			bridge.Stopping = true
			m.stopBridgeProcess(bridge)
			return socksURL, key, nil
		}
		go m.watchBridge(bridge, key)
		return fmt.Sprintf("socks5://127.0.0.1:%d", port), key, nil
	}
	return "", "", fmt.Errorf("xray chain bridge start failed: %w", lastErr)
}

func (m *XrayManager) buildChainRuntimeConfig(key string, runtime CompiledXrayChain, port int) (string, error) {
	baseDir := m.resolveWorkdir(key)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", err
	}
	cfg := map[string]interface{}{"log": map[string]interface{}{"loglevel": "info", "error": filepath.Join(baseDir, "xray-error.log")}, "inbounds": []interface{}{map[string]interface{}{"tag": "socks-in", "port": port, "listen": "127.0.0.1", "protocol": "socks", "settings": map[string]interface{}{"udp": true}, "sniffing": map[string]interface{}{"enabled": false}}}, "outbounds": append(append([]interface{}{}, toAnySlice(runtime.Outbounds)...), map[string]interface{}{"protocol": "direct", "tag": "direct"}, map[string]interface{}{"protocol": "blackhole", "tag": "block"}), "routing": map[string]interface{}{"rules": []interface{}{map[string]interface{}{"type": "field", "inboundTag": []string{"socks-in"}, "outboundTag": runtime.EntryTag}}}}
	if dnsCfg := parseDnsConfig(runtime.DNSServers); dnsCfg != nil {
		cfg["dns"] = dnsCfg
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	cfgPath := filepath.Join(baseDir, "xray-config.json")
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return "", err
	}
	return cfgPath, nil
}

func (m *SingBoxManager) EnsureChainBridge(chain ResolvedChain) (string, error) {
	compiled, err := CompileSingBoxChain(chain)
	if err != nil {
		return "", err
	}
	payload, _ := json.Marshal(compiled.Outbounds)
	key := computeNodeKey(string(payload))
	if socksURL, reused := m.tryReuseBridge(key); reused {
		return socksURL, nil
	}
	binaryPath, err := m.resolveBinary()
	if err != nil {
		return "", err
	}
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		port, portErr := nextAvailablePort()
		if portErr != nil {
			lastErr = portErr
			continue
		}
		cfgPath, cfgErr := m.buildChainConfig(key, compiled, port)
		if cfgErr != nil {
			return "", cfgErr
		}
		cmd := exec.Command(binaryPath, "run", "-c", cfgPath)
		hideWindow(cmd)
		cmd.Dir = filepath.Dir(cfgPath)
		if err := cmd.Start(); err != nil {
			lastErr = err
			continue
		}
		bridge := &SingBoxBridge{NodeKey: key, Port: port, Cmd: cmd, Pid: cmd.Process.Pid, Running: true}
		if err := waitPortReady("127.0.0.1", port, 10*time.Second); err != nil {
			bridge.Stopping = true
			m.stopBridgeProcess(bridge)
			lastErr = err
			continue
		}
		if socksURL, reused := m.registerBridge(key, bridge); reused {
			bridge.Stopping = true
			m.stopBridgeProcess(bridge)
			return socksURL, nil
		}
		go m.watchBridge(bridge, key)
		return fmt.Sprintf("socks5://127.0.0.1:%d", port), nil
	}
	return "", fmt.Errorf("sing-box chain bridge start failed: %w", lastErr)
}

func (m *SingBoxManager) buildChainConfig(key string, runtime CompiledSingBoxChain, port int) (string, error) {
	baseDir := m.resolveWorkdir(key)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", err
	}
	cfg := map[string]interface{}{"log": map[string]interface{}{"level": "info", "output": filepath.Join(baseDir, "singbox.log"), "timestamp": true}, "inbounds": []interface{}{map[string]interface{}{"type": "socks", "tag": "socks-in", "listen": "127.0.0.1", "listen_port": port}}, "outbounds": append(append([]interface{}{}, toAnySlice(runtime.Outbounds)...), map[string]interface{}{"type": "direct", "tag": "direct"}), "route": map[string]interface{}{"rules": []interface{}{map[string]interface{}{"inbound": []string{"socks-in"}, "outbound": runtime.EntryTag}}}}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	cfgPath := filepath.Join(baseDir, "singbox-config.json")
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return "", err
	}
	return cfgPath, nil
}

func toAnySlice(items []map[string]interface{}) []interface{} {
	out := make([]interface{}, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}
