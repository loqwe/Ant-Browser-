package backend

import (
	"fmt"
	"net"
	"strings"
	"time"
)

const browserStartReadyTimeout = 10 * time.Second
const browserStartStableWindow = 1200 * time.Millisecond

func waitBrowserDebugPortReady(debugPort int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if canConnectDebugPort(debugPort, 250*time.Millisecond) {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}

	return fmt.Errorf("浏览器进程未在 %s 内完成启动，调试端口 %d 未就绪", timeout.Round(time.Second), debugPort)
}

func waitBrowserDebugPortStable(debugPort int, timeout time.Duration, stableFor time.Duration) error {
	if err := waitBrowserDebugPortReady(debugPort, timeout); err != nil {
		return err
	}
	if stableFor <= 0 {
		return nil
	}

	deadline := time.Now().Add(stableFor)
	for time.Now().Before(deadline) {
		if !canConnectDebugPort(debugPort, 250*time.Millisecond) {
			return fmt.Errorf("浏览器调试端口 %d 短暂就绪后又失效", debugPort)
		}
		time.Sleep(150 * time.Millisecond)
	}
	return nil
}

func canConnectDebugPort(debugPort int, dialTimeout time.Duration) bool {
	if debugPort <= 0 {
		return false
	}

	address := fmt.Sprintf("127.0.0.1:%d", debugPort)
	conn, err := net.DialTimeout("tcp", address, dialTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func describeChromeProcessStartError(chromeBinaryPath string, err error) string {
	raw := strings.TrimSpace(err.Error())
	lower := strings.ToLower(raw)

	switch {
	case strings.Contains(lower, "access is denied"),
		strings.Contains(lower, "permission denied"),
		strings.Contains(raw, "拒绝访问"):
		return fmt.Sprintf("实例启动失败：系统拒绝启动浏览器进程。可执行文件：%s。请检查文件权限、杀毒软件拦截，或尝试以管理员身份运行。", chromeBinaryPath)
	case strings.Contains(lower, "not a valid win32 application"),
		strings.Contains(raw, "不是有效的 win32 应用程序"),
		strings.Contains(raw, "不是有效的 Win32 应用程序"),
		strings.Contains(raw, "bad exe format"),
		strings.Contains(lower, "exec format error"),
		strings.Contains(lower, "cannot execute binary file"):
		return fmt.Sprintf("实例启动失败：当前浏览器内核与系统/架构不兼容。可执行文件：%s。请确认 Linux 环境使用的是对应架构的 Chrome 内核，而不是 Windows 可执行文件。", chromeBinaryPath)
	case strings.Contains(raw, "系统找不到指定的文件"),
		strings.Contains(lower, "file not found"),
		strings.Contains(lower, "no such file"),
		strings.Contains(lower, "cannot find the file"):
		return fmt.Sprintf("实例启动失败：浏览器可执行文件不存在。可执行文件：%s。请检查内核路径是否正确，或重新下载内核。", chromeBinaryPath)
	case strings.Contains(raw, "目录名称无效"),
		strings.Contains(lower, "directory name is invalid"):
		return fmt.Sprintf("实例启动失败：浏览器工作目录无效。当前目录：%s。请检查内核路径配置是否正确。", chromeBinaryPath)
	default:
		return fmt.Sprintf("实例启动失败：浏览器进程拉起失败。可执行文件：%s。原因：%s。请检查内核文件是否完整、启动参数是否正确，或是否被安全软件拦截。", chromeBinaryPath, raw)
	}
}

func describeBrowserReadyTimeout(debugPort int, timeout time.Duration) string {
	return fmt.Sprintf("实例启动失败：浏览器进程已拉起，但在 %s 内未完成就绪，调试端口 %d 未开启。请检查内核文件是否完整、启动参数是否正确，或是否被安全软件拦截。", timeout.Round(time.Second), debugPort)
}
