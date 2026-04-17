package proxy

import (
	"net"
	"os/exec"
	"sync"
	"time"

	"ant-chrome/backend/internal/config"
	C "github.com/metacubex/mihomo/constant"
)

const (
	mihomoBridgeIdleTTL         = 45 * time.Second
	mihomoBridgeCleanupInterval = 15 * time.Second
)

type MihomoBridge struct {
	NodeKey    string
	Port       int
	Listener   net.Listener
	Proxy      C.Proxy
	Cmd        *exec.Cmd
	Pid        int
	Backend    string
	RefCount   int
	LastUsedAt time.Time
	Running    bool
}

type MihomoManager struct {
	Config   *config.Config
	AppRoot  string
	Bridges  map[string]*MihomoBridge
	mu       sync.Mutex
	stopCh   chan struct{}
	stopOnce sync.Once
}

func NewMihomoManager(cfg *config.Config, appRoot string) *MihomoManager {
	manager := &MihomoManager{Config: cfg, AppRoot: appRoot, Bridges: make(map[string]*MihomoBridge), stopCh: make(chan struct{})}
	go manager.cleanupLoop()
	return manager
}

func (m *MihomoManager) EnsureBridge(proxyConfig string) (string, error) {
	socksURL, _, err := m.ensureBridge(computeNodeKey("mihomo:\x00"+proxyConfig), func() (C.Proxy, error) { return buildMihomoProxy(proxyConfig, nil) }, func(port int) ([]byte, error) { return buildExternalBridgeConfig(proxyConfig, port) }, false)
	return socksURL, err
}

func (m *MihomoManager) AcquireBridge(proxyConfig string) (string, string, error) {
	return m.ensureBridge(computeNodeKey("mihomo:\x00"+proxyConfig), func() (C.Proxy, error) { return buildMihomoProxy(proxyConfig, nil) }, func(port int) ([]byte, error) { return buildExternalBridgeConfig(proxyConfig, port) }, true)
}

func (m *MihomoManager) EnsureChainBridge(chain ResolvedChain) (string, error) {
	socksURL, _, err := m.ensureBridge(computeNodeKey(chainKey(chain)), func() (C.Proxy, error) { return buildMihomoChainProxy(chain) }, func(port int) ([]byte, error) { return buildExternalChainBridgeConfig(chain, port) }, false)
	return socksURL, err
}

func (m *MihomoManager) AcquireChainBridge(chain ResolvedChain) (string, string, error) {
	return m.ensureBridge(computeNodeKey(chainKey(chain)), func() (C.Proxy, error) { return buildMihomoChainProxy(chain) }, func(port int) ([]byte, error) { return buildExternalChainBridgeConfig(chain, port) }, true)
}

func (m *MihomoManager) ReleaseBridge(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if bridge := m.Bridges[key]; bridge != nil && bridge.RefCount > 0 {
		bridge.RefCount--
		bridge.LastUsedAt = time.Now()
	}
}

func (m *MihomoManager) StopAll() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
		m.mu.Lock()
		defer m.mu.Unlock()
		for key, bridge := range m.Bridges {
			stopMihomoBridge(bridge)
			delete(m.Bridges, key)
		}
	})
}

func chainKey(chain ResolvedChain) string {
	key := "mihomo-chain"
	for _, hop := range chain.Hops {
		key += "\x00" + chainHopConfig(hop)
	}
	return key
}

func stopMihomoBridge(bridge *MihomoBridge) {
	if bridge == nil {
		return
	}
	bridge.Running = false
	if bridge.Listener != nil {
		_ = bridge.Listener.Close()
	}
	if bridge.Proxy != nil {
		_ = bridge.Proxy.Close()
	}
	if bridge.Cmd != nil && bridge.Cmd.Process != nil {
		_ = bridge.Cmd.Process.Kill()
	}
}

func (m *MihomoManager) cleanupLoop() {
	ticker := time.NewTicker(mihomoBridgeCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.cleanupIdleBridges()
		}
	}
}

func (m *MihomoManager) cleanupIdleBridges() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, bridge := range m.Bridges {
		if bridge == nil || bridge.RefCount > 0 || time.Since(bridge.LastUsedAt) < mihomoBridgeIdleTTL {
			continue
		}
		stopMihomoBridge(bridge)
		delete(m.Bridges, key)
	}
}
