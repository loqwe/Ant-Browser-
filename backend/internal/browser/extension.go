package browser

import (
	"archive/zip"
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"ant-chrome/backend/internal/logger"
	"github.com/google/uuid"
)

type extensionManifest struct {
	Name            string            `json:"name"`
	DefaultLocale   string            `json:"default_locale"`
	Version         string            `json:"version"`
	Description     string            `json:"description"`
	Permissions     []string          `json:"permissions"`
	HostPermissions []string          `json:"host_permissions"`
	OptionsPage     string            `json:"options_page"`
	Icons           map[string]string `json:"icons"`
}

type extensionLocaleEntry struct {
	Message string `json:"message"`
}

func isManifestMessagePlaceholder(value string) bool {
	return strings.HasPrefix(value, "__MSG_") && strings.HasSuffix(value, "__")
}

func resolveManifestLocalizedValue(baseDir, defaultLocale, value string) string {
	if !isManifestMessagePlaceholder(value) {
		return value
	}
	key := strings.TrimSuffix(strings.TrimPrefix(value, "__MSG_"), "__")
	for _, locale := range extensionLocaleCandidates(baseDir, defaultLocale) {
		if msg, ok := loadLocaleMessage(baseDir, locale, key); ok {
			return msg
		}
	}
	return value
}

func extensionLocaleCandidates(baseDir, defaultLocale string) []string {
	seen := map[string]struct{}{}
	ordered := make([]string, 0, 4)
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		ordered = append(ordered, v)
	}
	add(defaultLocale)
	add("en")
	entries, err := os.ReadDir(filepath.Join(baseDir, "_locales"))
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				add(entry.Name())
			}
		}
	}
	return ordered
}

func loadLocaleMessage(baseDir, locale, key string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(baseDir, "_locales", locale, "messages.json"))
	if err != nil {
		return "", false
	}
	messages := map[string]extensionLocaleEntry{}
	if err := json.Unmarshal(data, &messages); err != nil {
		return "", false
	}
	item, ok := messages[key]
	if !ok || strings.TrimSpace(item.Message) == "" {
		return "", false
	}
	return item.Message, true
}

func resolveManifestLocale(baseDir string, manifest extensionManifest) extensionManifest {
	manifest.Name = resolveManifestLocalizedValue(baseDir, manifest.DefaultLocale, manifest.Name)
	manifest.Description = resolveManifestLocalizedValue(baseDir, manifest.DefaultLocale, manifest.Description)
	return manifest
}

func isGlobalExtensionSourceType(sourceType string) bool {
	switch strings.ToLower(strings.TrimSpace(sourceType)) {
	case "crx", "zip":
		return true
	default:
		return false
	}
}

func (m *Manager) ExtensionsRoot() string {
	return m.ResolveRelativePath(filepath.Join("data", "extensions"))
}

func (m *Manager) ListExtensions() []Extension {
	if m.ExtensionDAO == nil {
		return nil
	}
	items, err := m.ExtensionDAO.List()
	if err != nil {
		return nil
	}
	result := make([]Extension, 0, len(items))
	for _, item := range items {
		result = append(result, *m.normalizeStoredExtension(item))
	}
	return result
}

func (m *Manager) ImportExtensionDir(dir string) (*Extension, error) {
	return m.importExtensionFromDir(dir, "unpacked", dir)
}

func (m *Manager) ImportExtensionCRX(path string) (*Extension, error) {
	log := logger.New("Extension")
	log.Info("开始导入扩展文件", logger.F("path", path))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取扩展文件失败: %w", err)
	}
	zipData, sourceType, err := extractExtensionArchivePayload(data, path)
	if err != nil {
		return nil, err
	}
	tempDir := filepath.Join(os.TempDir(), "ant-browser-ext-"+uuid.NewString())
	log.Debug("扩展文件临时解包目录", logger.F("temp_dir", tempDir), logger.F("source_type", sourceType))
	defer os.RemoveAll(tempDir)
	if err := extractZipToDir(zipData, tempDir); err != nil {
		return nil, err
	}
	resolvedDir, err := resolveExtractedExtensionDir(tempDir)
	if err != nil {
		return nil, err
	}
	item, err := m.importExtensionFromDir(resolvedDir, sourceType, path)
	if err != nil {
		log.Error("扩展文件入库失败", logger.F("path", path), logger.F("error", err.Error()))
		return nil, err
	}
	log.Info("扩展文件导入完成", logger.F("path", path), logger.F("source_type", sourceType), logger.F("extension_id", item.ExtensionId), logger.F("name", item.Name))
	return item, nil
}

func (m *Manager) ImportExtensionURL(url string) (*Extension, error) {
	log := logger.New("Extension")
	log.Info("开始从 URL 导入扩展", logger.F("url", url))
	data, contentType, statusCode, err := downloadExtensionBytes(url)
	if err != nil {
		return nil, fmt.Errorf("下载扩展文件失败: %w", err)
	}
	log.Info("扩展下载响应成功", logger.F("url", url), logger.F("status", statusCode), logger.F("content_type", contentType), logger.F("size", len(data)))
	zipData, sourceType, err := extractExtensionArchivePayload(data, url)
	if err != nil {
		return nil, err
	}
	tempDir := filepath.Join(os.TempDir(), "ant-browser-ext-"+uuid.NewString())
	log.Debug("URL 临时解包目录", logger.F("temp_dir", tempDir), logger.F("source_type", sourceType))
	defer os.RemoveAll(tempDir)
	if err := extractZipToDir(zipData, tempDir); err != nil {
		return nil, err
	}
	resolvedDir, err := resolveExtractedExtensionDir(tempDir)
	if err != nil {
		return nil, err
	}
	item, err := m.importExtensionFromDir(resolvedDir, sourceType, url)
	if err != nil {
		log.Error("URL 扩展入库失败", logger.F("url", url), logger.F("error", err.Error()))
		return nil, err
	}
	log.Info("URL 扩展导入完成", logger.F("url", url), logger.F("source_type", sourceType), logger.F("extension_id", item.ExtensionId), logger.F("name", item.Name))
	return item, nil
}

func (m *Manager) DeleteExtension(extensionId string) error {
	if m.ExtensionDAO == nil {
		return fmt.Errorf("????????")
	}
	item, err := m.ExtensionDAO.GetById(extensionId)
	if err != nil {
		return err
	}
	if err := m.ExtensionDAO.Delete(extensionId); err != nil {
		return err
	}
	if item.UnpackedPath != "" {
		_ = os.RemoveAll(item.UnpackedPath)
	}
	return nil
}

func (m *Manager) SetExtensionEnabledByDefault(extensionId string, enabled bool) (*Extension, error) {
	if m.ExtensionDAO == nil {
		return nil, fmt.Errorf("????????")
	}
	item, err := m.ExtensionDAO.GetById(extensionId)
	if err != nil {
		return nil, err
	}
	normalized := m.normalizeStoredExtension(item)
	normalized.EnabledByDefault = enabled
	if err := m.ExtensionDAO.Upsert(normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func (m *Manager) RefreshExtension(extensionId string) (*Extension, error) {
	if m.ExtensionDAO == nil {
		return nil, fmt.Errorf("????????")
	}
	item, err := m.ExtensionDAO.GetById(extensionId)
	if err != nil {
		return nil, err
	}
	normalized := m.normalizeStoredExtension(item)
	sourcePath := strings.TrimSpace(normalized.SourcePath)
	if sourcePath == "" {
		return nil, fmt.Errorf("?????????????")
	}
	if normalized.SourceType == "unpacked" {
		resolvedDir, err := resolveExtractedExtensionDir(sourcePath)
		if err != nil {
			return nil, err
		}
		return m.replaceExtensionFromDir(normalized, resolvedDir, normalized.SourceType, sourcePath)
	}
	if strings.HasPrefix(strings.ToLower(sourcePath), "http://") || strings.HasPrefix(strings.ToLower(sourcePath), "https://") {
		data, _, _, err := downloadExtensionBytes(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("????????: %w", err)
		}
		return m.refreshExtensionFromArchive(normalized, data, sourcePath)
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("????????: %w", err)
	}
	return m.refreshExtensionFromArchive(normalized, data, sourcePath)
}

func (m *Manager) refreshExtensionFromArchive(existing *Extension, data []byte, sourcePath string) (*Extension, error) {
	zipData, sourceType, err := extractExtensionArchivePayload(data, sourcePath)
	if err != nil {
		return nil, err
	}
	tempDir := filepath.Join(os.TempDir(), "ant-browser-ext-refresh-"+uuid.NewString())
	defer os.RemoveAll(tempDir)
	if err := extractZipToDir(zipData, tempDir); err != nil {
		return nil, err
	}
	resolvedDir, err := resolveExtractedExtensionDir(tempDir)
	if err != nil {
		return nil, err
	}
	return m.replaceExtensionFromDir(existing, resolvedDir, sourceType, sourcePath)
}

func (m *Manager) replaceExtensionFromDir(existing *Extension, dir string, sourceType string, sourcePath string) (*Extension, error) {
	manifestPath := filepath.Join(dir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("?? manifest.json ??: %w", err)
	}
	var manifest extensionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("?? manifest.json ??: %w", err)
	}
	manifest = resolveManifestLocale(dir, manifest)
	if manifest.Name == "" {
		return nil, fmt.Errorf("manifest.json ?? name")
	}
	if manifest.Version == "" {
		return nil, fmt.Errorf("manifest.json ?? version")
	}
	stored := strings.TrimSpace(existing.UnpackedPath)
	if stored == "" {
		stored = filepath.Join(m.ExtensionsRoot(), existing.ExtensionId)
	}
	_ = os.RemoveAll(stored)
	if err := os.MkdirAll(stored, 0o755); err != nil {
		return nil, fmt.Errorf("??????????: %w", err)
	}
	if err := copyTree(dir, stored); err != nil {
		return nil, fmt.Errorf("????????: %w", err)
	}
	item := &Extension{
		ExtensionId:      existing.ExtensionId,
		Name:             manifest.Name,
		SourceType:       sourceType,
		SourcePath:       sourcePath,
		UnpackedPath:     stored,
		Version:          manifest.Version,
		Description:      manifest.Description,
		Permissions:      append([]string{}, manifest.Permissions...),
		HostPermissions:  append([]string{}, manifest.HostPermissions...),
		OptionsPage:      manifest.OptionsPage,
		IconPath:         chooseManifestIcon(manifest.Icons),
		EnabledByDefault: existing.EnabledByDefault,
		CreatedAt:        existing.CreatedAt,
	}
	if err := m.ExtensionDAO.Upsert(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (m *Manager) ListProfileExtensions(profileId string) []ProfileExtensionBinding {
	if m.ProfileExtensionDAO == nil {
		return nil
	}
	items, err := m.ProfileExtensionDAO.ListByProfile(profileId)
	if err != nil {
		return nil
	}
	result := make([]ProfileExtensionBinding, 0, len(items))
	for _, item := range items {
		result = append(result, *item)
	}
	return result
}

func (m *Manager) SaveProfileExtensions(profileId string, bindings []ProfileExtensionBinding) error {
	items := make([]*ProfileExtensionBinding, 0, len(bindings))
	for i := range bindings {
		item := bindings[i]
		if item.BindingId == "" {
			item.BindingId = "bind-" + uuid.NewString()
		}
		item.ProfileId = profileId
		items = append(items, &item)
	}
	return m.ProfileExtensionDAO.ReplaceProfileBindings(profileId, items)
}

func (m *Manager) ExtensionLoadPaths(profileId string) ([]string, error) {
	if m.ExtensionDAO == nil {
		return nil, nil
	}
	paths := make([]string, 0)
	seen := make(map[string]struct{})

	items, err := m.ExtensionDAO.List()
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		ext := m.normalizeStoredExtension(item)
		if ext == nil || !ext.EnabledByDefault || ext.UnpackedPath == "" {
			continue
		}
		if _, ok := seen[ext.ExtensionId]; ok {
			continue
		}
		paths = append(paths, ext.UnpackedPath)
		seen[ext.ExtensionId] = struct{}{}
	}

	if m.ProfileExtensionDAO == nil {
		return paths, nil
	}
	bindings, err := m.ProfileExtensionDAO.ListByProfile(profileId)
	if err != nil {
		return nil, err
	}
	for _, binding := range bindings {
		if !binding.Enabled {
			continue
		}
		if _, ok := seen[binding.ExtensionId]; ok {
			continue
		}
		ext, err := m.ExtensionDAO.GetById(binding.ExtensionId)
		if err != nil {
			continue
		}
		ext = m.normalizeStoredExtension(ext)
		if ext == nil || ext.UnpackedPath == "" {
			continue
		}
		paths = append(paths, ext.UnpackedPath)
		seen[ext.ExtensionId] = struct{}{}
	}
	return paths, nil
}

func (m *Manager) importExtensionFromDir(dir string, sourceType string, sourcePath string) (*Extension, error) {
	log := logger.New("Extension")
	manifestPath := filepath.Join(dir, "manifest.json")
	log.Info("开始解析扩展目录", logger.F("source_type", sourceType), logger.F("source_path", sourcePath), logger.F("dir", dir))
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("读取 manifest.json 失败: %w", err)
	}
	var manifest extensionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("解析 manifest.json 失败: %w", err)
	}
	manifest = resolveManifestLocale(dir, manifest)
	if manifest.Name == "" {
		return nil, fmt.Errorf("manifest.json 缺少 name")
	}
	if manifest.Version == "" {
		return nil, fmt.Errorf("manifest.json 缺少 version")
	}
	id := "ext-" + uuid.NewString()
	root := m.ExtensionsRoot()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("创建扩展仓库目录失败: %w", err)
	}
	stored := filepath.Join(root, id)
	if err := copyTree(dir, stored); err != nil {
		return nil, fmt.Errorf("复制扩展目录失败: %w", err)
	}
	item := &Extension{ExtensionId: id, Name: manifest.Name, SourceType: sourceType, SourcePath: sourcePath, UnpackedPath: stored, Version: manifest.Version, Description: manifest.Description, Permissions: append([]string{}, manifest.Permissions...), HostPermissions: append([]string{}, manifest.HostPermissions...), OptionsPage: manifest.OptionsPage, IconPath: chooseManifestIcon(manifest.Icons), EnabledByDefault: isGlobalExtensionSourceType(sourceType)}
	if err := m.ExtensionDAO.Upsert(item); err != nil {
		return nil, err
	}
	log.Info("扩展入库成功", logger.F("extension_id", item.ExtensionId), logger.F("name", item.Name), logger.F("stored_path", stored))
	return item, nil
}
func extractCRXZipPayload(data []byte) ([]byte, error) {
	if len(data) < 12 || string(data[:4]) != "Cr24" {
		return nil, fmt.Errorf("非法 CRX 文件：缺少 Cr24 头")
	}
	version := binary.LittleEndian.Uint32(data[4:8])
	switch version {
	case 2:
		if len(data) < 16 {
			return nil, fmt.Errorf("非法 CRX 文件：CRX2 头长度不足")
		}
		pubLen := binary.LittleEndian.Uint32(data[8:12])
		sigLen := binary.LittleEndian.Uint32(data[12:16])
		start := 16 + int(pubLen) + int(sigLen)
		if start > len(data) {
			return nil, fmt.Errorf("非法 CRX 文件：CRX2 数据越界")
		}
		return data[start:], nil
	case 3:
		headerLen := binary.LittleEndian.Uint32(data[8:12])
		start := 12 + int(headerLen)
		if start > len(data) {
			return nil, fmt.Errorf("非法 CRX 文件：CRX3 数据越界")
		}
		return data[start:], nil
	default:
		return nil, fmt.Errorf("不支持的 CRX 版本: %d", version)
	}
}

func extractExtensionArchivePayload(data []byte, sourcePath string) ([]byte, string, error) {
	if len(data) >= 4 && string(data[:4]) == "Cr24" {
		zipData, err := extractCRXZipPayload(data)
		if err != nil {
			return nil, "", err
		}
		return zipData, "crx", nil
	}
	if len(data) >= 4 && data[0] == 'P' && data[1] == 'K' {
		return data, "zip", nil
	}
	if strings.EqualFold(filepath.Ext(sourcePath), ".zip") {
		return nil, "", fmt.Errorf("非法 ZIP 文件：无法识别压缩结构")
	}
	return nil, "", fmt.Errorf("不支持的扩展文件格式，仅支持 .crx 或 .zip")
}

func resolveExtractedExtensionDir(root string) (string, error) {
	manifestPath := filepath.Join(root, "manifest.json")
	if _, err := os.Stat(manifestPath); err == nil {
		return root, nil
	}
	candidates := make([]string, 0, 4)
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == "__MACOSX" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(info.Name(), "manifest.json") {
			candidates = append(candidates, filepath.Dir(path))
		}
		return nil
	})
	if len(candidates) == 0 {
		return "", fmt.Errorf("???????????? manifest.json")
	}
	sort.Slice(candidates, func(i, j int) bool {
		return len(candidates[i]) < len(candidates[j])
	})
	return candidates[0], nil
}

func extractZipToDir(zipData []byte, dst string) error {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("解析 CRX 中的 ZIP 失败: %w", err)
	}
	for _, file := range reader.File {
		target := filepath.Join(dst, file.Name)
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("创建扩展目录失败: %w", err)
			}
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("读取压缩文件内容失败: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			rc.Close()
			return fmt.Errorf("创建扩展目录失败: %w", err)
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			rc.Close()
			return fmt.Errorf("写入扩展文件失败: %w", err)
		}
		_, copyErr := io.Copy(out, rc)
		closeErr := rc.Close()
		_ = out.Close()
		if copyErr != nil {
			return fmt.Errorf("解压扩展文件失败: %w", copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("关闭扩展压缩流失败: %w", closeErr)
		}
	}
	return nil
}

func (m *Manager) normalizeStoredExtension(item *Extension) *Extension {
	if item == nil {
		return item
	}
	clone := *item
	changed := false
	if clone.UnpackedPath != "" {
		data, err := os.ReadFile(filepath.Join(clone.UnpackedPath, "manifest.json"))
		if err == nil {
			var manifest extensionManifest
			if err := json.Unmarshal(data, &manifest); err == nil {
				manifest = resolveManifestLocale(clone.UnpackedPath, manifest)
				if isManifestMessagePlaceholder(clone.Name) && manifest.Name != "" && clone.Name != manifest.Name {
					clone.Name = manifest.Name
					changed = true
				}
				if isManifestMessagePlaceholder(clone.Description) && manifest.Description != "" && clone.Description != manifest.Description {
					clone.Description = manifest.Description
					changed = true
				}
			}
		}
	}
	if changed && m.ExtensionDAO != nil {
		_ = m.ExtensionDAO.Upsert(&clone)
	}
	return &clone
}

func chooseManifestIcon(icons map[string]string) string {
	if len(icons) == 0 {
		return ""
	}
	sizes := make([]int, 0, len(icons))
	for key := range icons {
		if n, err := strconv.Atoi(key); err == nil {
			sizes = append(sizes, n)
		}
	}
	sort.Ints(sizes)
	if len(sizes) > 0 {
		return icons[strconv.Itoa(sizes[len(sizes)-1])]
	}
	for _, value := range icons {
		return value
	}
	return ""
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		}
		return copyFileSimple(path, target, info.Mode().Perm())
	})
}

func copyFileSimple(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func BuildChromeWebStoreDownloadURL(extensionID string) string {
	return fmt.Sprintf("https://clients2.google.com/service/update2/crx?response=redirect&prodversion=142.0.0.0&acceptformat=crx2,crx3&x=id%%3D%s%%26installsource%%3Dondemand%%26uc", extensionID)
}

func (m *Manager) ImportChromeWebStoreExtension(extensionID string) (*Extension, error) {
	if extensionID == "" {
		return nil, fmt.Errorf("扩展 ID 不能为空")
	}
	return m.ImportExtensionURL(BuildChromeWebStoreDownloadURL(extensionID))
}

func downloadExtensionBytes(url string) ([]byte, string, int, error) {
	log := logger.New("Extension")
	if runtime.GOOS == "windows" {
		log.Info("开始使用 curl 下载扩展", logger.F("url", url))
		if data, ctype, status, err := downloadExtensionBytesWithCurl(url); err == nil {
			log.Info("curl 下载扩展成功", logger.F("url", url), logger.F("status", status), logger.F("size", len(data)))
			return data, ctype, status, nil
		} else {
			log.Warn("curl 下载扩展失败，准备回退 PowerShell", logger.F("url", url), logger.F("error", err.Error()))
		}

		log.Info("开始使用 PowerShell 下载扩展", logger.F("url", url))
		if data, ctype, status, err := downloadExtensionBytesWithPowerShell(url); err == nil {
			log.Info("PowerShell 下载扩展成功", logger.F("url", url), logger.F("status", status), logger.F("size", len(data)))
			return data, ctype, status, nil
		} else {
			log.Warn("PowerShell 下载扩展失败，准备回退 Go HTTP", logger.F("url", url), logger.F("error", err.Error()))
		}
	}

	client := &http.Client{
		Timeout: 45 * time.Second,
		Transport: &http.Transport{
			ForceAttemptHTTP2: false,
			TLSClientConfig:   &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		data, ctype, status, err := downloadExtensionBytesWithClient(client, url)
		if err == nil {
			log.Info("Go HTTP 下载扩展成功", logger.F("url", url), logger.F("status", status), logger.F("size", len(data)), logger.F("attempt", attempt))
			return data, ctype, status, nil
		}
		lastErr = err
		log.Warn("Go HTTP 下载扩展失败", logger.F("url", url), logger.F("error", err.Error()), logger.F("attempt", attempt))
		if !strings.Contains(strings.ToLower(err.Error()), "eof") {
			break
		}
		time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
	}
	return nil, "", 0, lastErr
}

func downloadExtensionBytesWithCurl(url string) ([]byte, string, int, error) {
	curlPath, err := exec.LookPath("curl.exe")
	if err != nil {
		return nil, "", 0, err
	}
	tempFile := filepath.Join(os.TempDir(), "antbrowser-ext-"+uuid.NewString()+".crx")
	defer os.Remove(tempFile)
	cmd := exec.Command(curlPath, "--http1.1", "-L", "--fail", "-A", "Mozilla/5.0 AntBrowser/1.1.0", "-H", "Accept: application/x-chrome-extension,application/octet-stream,*/*", "-o", tempFile, url)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, "", 0, fmt.Errorf("%v %s", err, strings.TrimSpace(string(output)))
	}
	data, err := os.ReadFile(tempFile)
	if err != nil {
		return nil, "", 0, err
	}
	return data, "application/x-chrome-extension", 200, nil
}

func downloadExtensionBytesWithClient(client *http.Client, url string) ([]byte, string, int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 AntBrowser/1.1.0")
	req.Header.Set("Accept", "application/x-chrome-extension,application/octet-stream,*/*")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header.Get("Content-Type"), resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.Header.Get("Content-Type"), resp.StatusCode, err
	}
	return data, resp.Header.Get("Content-Type"), resp.StatusCode, nil
}

func downloadExtensionBytesWithPowerShell(url string) ([]byte, string, int, error) {
	tempFile := filepath.Join(os.TempDir(), "antbrowser-ext-"+uuid.NewString()+".crx")
	defer os.Remove(tempFile)
	cmd := exec.Command("powershell.exe", "-Command", "$ProgressPreference='SilentlyContinue'; Invoke-WebRequest -UseBasicParsing -MaximumRedirection 5 -Uri $args[0] -OutFile $args[1] -Headers @{ 'User-Agent'='Mozilla/5.0 AntBrowser/1.1.0'; 'Accept'='application/x-chrome-extension,application/octet-stream,*/*' } | Out-Null", url, tempFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, "", 0, fmt.Errorf("%v %s", err, strings.TrimSpace(string(output)))
	}
	data, err := os.ReadFile(tempFile)
	if err != nil {
		return nil, "", 0, err
	}
	return data, "application/x-chrome-extension", 200, nil
}
