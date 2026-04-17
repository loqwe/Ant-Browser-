package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func (m *MihomoManager) tryReuseBridge(key string, pin bool) (string, bool) {
	var stale *MihomoBridge
	m.mu.Lock()
	if bridge := m.Bridges[key]; bridge != nil {
		if bridge.Running && waitPortReady("127.0.0.1", bridge.Port, 800*time.Millisecond) == nil {
			if pin {
				bridge.RefCount++
			}
			bridge.LastUsedAt = time.Now()
			socksURL := fmt.Sprintf("socks5://127.0.0.1:%d", bridge.Port)
			m.mu.Unlock()
			return socksURL, true
		}
		stale = bridge
		delete(m.Bridges, key)
	}
	m.mu.Unlock()
	if stale != nil {
		stopMihomoBridge(stale)
	}
	return "", false
}

func (m *MihomoManager) registerBridge(key string, bridge *MihomoBridge, pin bool) (string, bool) {
	var duplicate *MihomoBridge
	m.mu.Lock()
	if existing := m.Bridges[key]; existing != nil {
		if existing.Running && waitPortReady("127.0.0.1", existing.Port, 800*time.Millisecond) == nil {
			if pin {
				existing.RefCount++
			}
			existing.LastUsedAt = time.Now()
			duplicate = bridge
			socksURL := fmt.Sprintf("socks5://127.0.0.1:%d", existing.Port)
			m.mu.Unlock()
			if duplicate != nil {
				stopMihomoBridge(duplicate)
			}
			return socksURL, true
		}
		duplicate = existing
		delete(m.Bridges, key)
	}
	if pin {
		bridge.RefCount = 1
	}
	bridge.LastUsedAt = time.Now()
	m.Bridges[key] = bridge
	m.mu.Unlock()
	if duplicate != nil {
		stopMihomoBridge(duplicate)
	}
	return "", false
}

func (m *MihomoManager) startExternalCoreBridge(binaryPath, key string, buildConfig func(int) ([]byte, error), pin bool) (string, error) {
	port, err := nextAvailablePort()
	if err != nil {
		return "", err
	}
	cfg, err := buildConfig(port)
	if err != nil {
		return "", err
	}
	baseDir := m.externalWorkdir(key)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", err
	}
	cfgPath := filepath.Join(baseDir, "config.yaml")
	if err := os.WriteFile(cfgPath, cfg, 0644); err != nil {
		return "", err
	}
	cmd := exec.Command(binaryPath, "-f", cfgPath, "-d", baseDir)
	hideWindow(cmd)
	cmd.Dir = baseDir
	stderrPath := filepath.Join(baseDir, "mihomo-stderr.log")
	stderrFile, _ := os.Create(stderrPath)
	if stderrFile != nil {
		cmd.Stderr = stderrFile
		cmd.Stdout = stderrFile
	}
	if err := cmd.Start(); err != nil {
		if stderrFile != nil {
			stderrFile.Close()
		}
		return "", err
	}
	bridge := &MihomoBridge{NodeKey: key, Port: port, Cmd: cmd, Pid: cmd.Process.Pid, Backend: "external-core", LastUsedAt: time.Now(), Running: true}
	if err := waitPortReady("127.0.0.1", port, 10*time.Second); err != nil {
		if stderrFile != nil {
			stderrFile.Close()
		}
		stopMihomoBridge(bridge)
		return "", err
	}
	if stderrFile != nil {
		stderrFile.Close()
	}
	if socksURL, reused := m.registerBridge(key, bridge, pin); reused {
		return socksURL, nil
	}
	go m.watchExternalBridge(key, bridge)
	return fmt.Sprintf("socks5://127.0.0.1:%d", port), nil
}

func (m *MihomoManager) watchExternalBridge(key string, bridge *MihomoBridge) {
	if bridge == nil || bridge.Cmd == nil {
		return
	}
	_ = bridge.Cmd.Wait()
	m.mu.Lock()
	if current := m.Bridges[key]; current == bridge {
		delete(m.Bridges, key)
	}
	bridge.Running = false
	m.mu.Unlock()
}
