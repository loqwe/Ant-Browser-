package backend

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDescribeChromeProcessStartError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "file not found",
			err:  fmt.Errorf("fork/exec C:\\chrome.exe: The system cannot find the file specified."),
			want: "浏览器可执行文件不存在",
		},
		{
			name: "access denied",
			err:  fmt.Errorf("fork/exec C:\\chrome.exe: Access is denied."),
			want: "系统拒绝启动浏览器进程",
		},
		{
			name: "invalid win32",
			err:  fmt.Errorf("%%1 is not a valid Win32 application"),
			want: "与系统/架构不兼容",
		},
		{
			name: "linux exec format error",
			err:  fmt.Errorf("fork/exec /opt/chrome/chrome.exe: exec format error"),
			want: "与系统/架构不兼容",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := describeChromeProcessStartError(`C:\chrome.exe`, tt.err)
			if !strings.Contains(got, tt.want) {
				t.Fatalf("expected %q to contain %q", got, tt.want)
			}
		})
	}
}

func TestDescribeBrowserReadyTimeout(t *testing.T) {
	got := describeBrowserReadyTimeout(9222, 10*time.Second)
	if !strings.Contains(got, "调试端口 9222 未开启") {
		t.Fatalf("unexpected timeout message: %q", got)
	}
}
