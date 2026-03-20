package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"net"
	"os/exec"
	"reflect"
	goruntime "runtime"
	"testing"
	"time"
)

func TestEnsureNewWindowLaunchArgAddsFlagOnce(t *testing.T) {
	t.Parallel()

	got := ensureNewWindowLaunchArg([]string{"--lang=en-US"})
	want := []string{"--lang=en-US", "--new-window"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ensureNewWindowLaunchArg 结果错误: got=%v want=%v", got, want)
	}

	got = ensureNewWindowLaunchArg([]string{"--new-window", "--lang=en-US"})
	want = []string{"--new-window", "--lang=en-US"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ensureNewWindowLaunchArg 不应重复追加: got=%v want=%v", got, want)
	}
}

func TestIsBrowserProfileLive(t *testing.T) {
	t.Parallel()

	ln := mustListenLoopback(t)
	defer ln.Close()

	profile := &BrowserProfile{
		Running:   true,
		DebugPort: listenerPort(t, ln),
	}
	if !isBrowserProfileLive(profile) {
		t.Fatal("期望存活中的调试端口被识别为运行中实例")
	}

	if isBrowserProfileLive(&BrowserProfile{Running: true, DebugPort: 0}) {
		t.Fatal("debugPort=0 不应被识别为运行中实例")
	}
}

func TestWaitBrowserDebugPortStableKeepsListeningPort(t *testing.T) {
	t.Parallel()

	ln := mustListenLoopback(t)
	defer ln.Close()

	if err := waitBrowserDebugPortStable(listenerPort(t, ln), time.Second, 250*time.Millisecond); err != nil {
		t.Fatalf("waitBrowserDebugPortStable 返回错误: %v", err)
	}
}

func TestWaitBrowserDebugPortStableRejectsEphemeralPort(t *testing.T) {
	t.Parallel()

	ln := mustListenLoopback(t)
	port := listenerPort(t, ln)
	time.AfterFunc(120*time.Millisecond, func() {
		_ = ln.Close()
	})

	err := waitBrowserDebugPortStable(port, time.Second, 400*time.Millisecond)
	if err == nil {
		t.Fatal("期望短暂就绪后关闭的端口被判定为失败")
	}
}

func TestWaitBrowserProcessKeepsRunningWhileDebugPortAlive(t *testing.T) {
	ln := mustListenLoopback(t)
	port := listenerPort(t, ln)

	app := NewApp("")
	app.browserMgr = browser.NewManager(config.DefaultConfig(), "")
	app.browserMgr.Profiles = map[string]*BrowserProfile{
		"profile-detached": {
			ProfileId:   "profile-detached",
			ProfileName: "Detached Browser",
			Running:     true,
			DebugPort:   port,
			Pid:         12345,
		},
	}
	app.browserMgr.BrowserProcesses = make(map[string]*exec.Cmd)

	cmd := shortLivedCommand()
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动短命测试进程失败: %v", err)
	}
	app.browserMgr.BrowserProcesses["profile-detached"] = cmd

	done := make(chan struct{})
	go func() {
		app.waitBrowserProcess("profile-detached", cmd)
		close(done)
	}()

	waitForCondition(t, 3*time.Second, func() bool {
		app.browserMgr.Mutex.Lock()
		defer app.browserMgr.Mutex.Unlock()

		profile := app.browserMgr.Profiles["profile-detached"]
		_, tracked := app.browserMgr.BrowserProcesses["profile-detached"]
		return profile != nil && profile.Running && !tracked
	})

	_ = ln.Close()

	waitForCondition(t, 4*time.Second, func() bool {
		app.browserMgr.Mutex.Lock()
		defer app.browserMgr.Mutex.Unlock()

		profile := app.browserMgr.Profiles["profile-detached"]
		return profile != nil && !profile.Running && profile.DebugPort == 0 && profile.Pid == 0
	})

	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("waitBrowserProcess 未在调试端口关闭后结束")
	}
}

func mustListenLoopback(t *testing.T) net.Listener {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听测试端口失败: %v", err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	return ln
}

func listenerPort(t *testing.T, ln net.Listener) int {
	t.Helper()

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("解析监听地址失败: %T", ln.Addr())
	}
	return tcpAddr.Port
}

func shortLivedCommand() *exec.Cmd {
	if goruntime.GOOS == "windows" {
		return exec.Command("cmd", "/c", "exit", "0")
	}
	return exec.Command("sh", "-c", "exit 0")
}

func waitForCondition(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("等待条件成立超时")
}
