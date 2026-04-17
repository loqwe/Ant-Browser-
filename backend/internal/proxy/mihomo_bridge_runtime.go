package proxy

import (
	"fmt"
	"net"
	"time"

	C "github.com/metacubex/mihomo/constant"
)

func (m *MihomoManager) ensureBridge(key string, build func() (C.Proxy, error), buildExternal func(int) ([]byte, error), pin bool) (string, string, error) {
	if socksURL, reused := m.tryReuseBridge(key, pin); reused {
		return socksURL, key, nil
	}
	if buildExternal != nil {
		if binaryPath, found, err := m.resolveExternalBinary(); err != nil {
			return "", "", err
		} else if found {
			socksURL, err := m.startExternalCoreBridge(binaryPath, key, buildExternal, pin)
			return socksURL, key, err
		}
	}
	return m.startEmbeddedBridge(key, build, pin)
}

func (m *MihomoManager) startEmbeddedBridge(key string, build func() (C.Proxy, error), pin bool) (string, string, error) {
	proxyInstance, err := build()
	if err != nil {
		return "", "", err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = proxyInstance.Close()
		return "", "", err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	bridge := &MihomoBridge{NodeKey: key, Port: port, Listener: listener, Proxy: proxyInstance, Backend: "embedded", LastUsedAt: time.Now(), Running: true}
	if socksURL, reused := m.registerBridge(key, bridge, pin); reused {
		stopMihomoBridge(bridge)
		return socksURL, key, nil
	}
	go serveMihomoBridge(listener, func() {
		m.mu.Lock()
		if current := m.Bridges[key]; current != nil {
			current.LastUsedAt = time.Now()
		}
		m.mu.Unlock()
	}, proxyInstance)
	return fmt.Sprintf("socks5://127.0.0.1:%d", port), key, nil
}
