package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"ant-chrome/backend/internal/fsutil"
)

func (m *MihomoManager) resolveExternalBinary() (string, bool, error) {
	candidates := []string{}
	if path := strings.TrimSpace(m.Config.Browser.ClashBinaryPath); path != "" {
		candidates = append(candidates, resolveEnvPath(path, m.AppRoot))
	}
	if path := strings.TrimSpace(os.Getenv("CLASH_BINARY_PATH")); path != "" {
		candidates = append(candidates, path)
	}
	platformDir := fmt.Sprintf("%s-%s", goruntime.GOOS, goruntime.GOARCH)
	binaryNames := []string{"mihomo", "clash-meta", "clash"}
	if goruntime.GOOS == "windows" {
		binaryNames = []string{"mihomo.exe", "clash-meta.exe", "clash.exe"}
	}
	searchDirs := []string{}
	if m.AppRoot != "" {
		searchDirs = append(searchDirs, filepath.Join(m.AppRoot, "bin", platformDir), filepath.Join(m.AppRoot, "bin"))
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		searchDirs = append(searchDirs, filepath.Join(exeDir, "bin", platformDir), filepath.Join(exeDir, "bin"))
	}
	if goruntime.GOOS == "windows" {
		roots := []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Clash Verge"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Clash Verge Rev"),
			filepath.Join(os.Getenv("ProgramFiles"), "Clash Verge"),
			filepath.Join(os.Getenv("ProgramFiles"), "Clash Verge Rev"),
		}
		for _, root := range roots {
			searchDirs = append(searchDirs, filepath.Join(root, "resources"), filepath.Join(root, "resources", "mihomo"), root)
		}
	}
	for _, dir := range searchDirs {
		for _, name := range binaryNames {
			candidates = append(candidates, filepath.Join(dir, name))
		}
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			if err := fsutil.EnsureExecutable(candidate); err != nil {
				return "", true, err
			}
			return candidate, true, nil
		}
	}
	for _, name := range binaryNames {
		if path, err := exec.LookPath(name); err == nil {
			if err := fsutil.EnsureExecutable(path); err != nil {
				return "", true, err
			}
			return path, true, nil
		}
	}
	return "", false, nil
}
