package browser

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestImportExtensionCRXUnpacksIntoRepository(t *testing.T) {
	t.Parallel()
	db := newExtensionTestDB(t)
	mgr := NewManager(&config.Config{}, t.TempDir())
	mgr.ExtensionDAO = NewSQLiteExtensionDAO(db.GetConn())

	crxPath := filepath.Join(t.TempDir(), "demo.crx")
	if err := os.WriteFile(crxPath, buildTestCRX(t, `{"name":"测试扩展","version":"1.2.3","description":"自动解包测试"}`), 0o644); err != nil {
		t.Fatalf("写入测试 CRX 失败: %v", err)
	}

	item, err := mgr.ImportExtensionCRX(crxPath)
	if err != nil {
		t.Fatalf("导入 CRX 失败: %v", err)
	}
	if item.SourceType != "crx" {
		t.Fatalf("SourceType 应为 crx，实际=%s", item.SourceType)
	}
	if item.Name != "测试扩展" || item.Version != "1.2.3" {
		t.Fatalf("扩展元信息不正确: %+v", item)
	}
	manifestPath := filepath.Join(item.UnpackedPath, "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("解包后的 manifest.json 不存在: %v", err)
	}
}

func TestImportExtensionCRXRejectsInvalidFile(t *testing.T) {
	t.Parallel()
	db := newExtensionTestDB(t)
	mgr := NewManager(&config.Config{}, t.TempDir())
	mgr.ExtensionDAO = NewSQLiteExtensionDAO(db.GetConn())

	badPath := filepath.Join(t.TempDir(), "bad.crx")
	if err := os.WriteFile(badPath, []byte("not-a-crx"), 0o644); err != nil {
		t.Fatalf("写入非法 CRX 失败: %v", err)
	}

	if _, err := mgr.ImportExtensionCRX(badPath); err == nil {
		t.Fatal("期望非法 CRX 导入失败")
	}
}

func buildTestZipArchive(t *testing.T, manifest string) []byte {
	t.Helper()
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	w, err := zw.Create("manifest.json")
	if err != nil {
		t.Fatalf("?? manifest entry ??: %v", err)
	}
	if _, err := w.Write([]byte(manifest)); err != nil {
		t.Fatalf("?? manifest ????: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("?? zip writer ??: %v", err)
	}
	return zipBuf.Bytes()
}

func buildTestZipArchiveWithRootDir(t *testing.T, rootDir string, manifest string) []byte {
	t.Helper()
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	w, err := zw.Create(rootDir + "/manifest.json")
	if err != nil {
		t.Fatalf("?? manifest entry ??: %v", err)
	}
	if _, err := w.Write([]byte(manifest)); err != nil {
		t.Fatalf("?? manifest ????: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("?? zip writer ??: %v", err)
	}
	return zipBuf.Bytes()
}

func buildTestCRX(t *testing.T, manifest string) []byte {
	t.Helper()
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	w, err := zw.Create("manifest.json")
	if err != nil {
		t.Fatalf("创建 manifest entry 失败: %v", err)
	}
	if _, err := w.Write([]byte(manifest)); err != nil {
		t.Fatalf("写入 manifest 内容失败: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("关闭 zip writer 失败: %v", err)
	}

	var crx bytes.Buffer
	crx.Write([]byte{'C', 'r', '2', '4'})
	_ = binary.Write(&crx, binary.LittleEndian, uint32(2))
	_ = binary.Write(&crx, binary.LittleEndian, uint32(0))
	_ = binary.Write(&crx, binary.LittleEndian, uint32(0))
	crx.Write(zipBuf.Bytes())
	return crx.Bytes()
}

func TestImportExtensionCRXSupportsZipArchive(t *testing.T) {
	t.Parallel()
	db := newExtensionTestDB(t)
	mgr := NewManager(&config.Config{}, t.TempDir())
	mgr.ExtensionDAO = NewSQLiteExtensionDAO(db.GetConn())

	zipPath := filepath.Join(t.TempDir(), "demo.zip")
	if err := os.WriteFile(zipPath, buildTestZipArchive(t, `{"name":"Zip ??","version":"2.0.0","description":"zip ????"}`), 0o644); err != nil {
		t.Fatalf("???? ZIP ??: %v", err)
	}

	item, err := mgr.ImportExtensionCRX(zipPath)
	if err != nil {
		t.Fatalf("?? ZIP ????: %v", err)
	}
	if item.SourceType != "zip" {
		t.Fatalf("SourceType ?? zip???=%s", item.SourceType)
	}
	if item.Name != "Zip ??" || item.Version != "2.0.0" {
		t.Fatalf("ZIP ????????: %+v", item)
	}
	manifestPath := filepath.Join(item.UnpackedPath, "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("ZIP ???? manifest.json ???: %v", err)
	}
}

func TestImportExtensionCRXSupportsZipArchiveWithNestedRoot(t *testing.T) {
	t.Parallel()
	db := newExtensionTestDB(t)
	mgr := NewManager(&config.Config{}, t.TempDir())
	mgr.ExtensionDAO = NewSQLiteExtensionDAO(db.GetConn())

	zipPath := filepath.Join(t.TempDir(), "demo-nested.zip")
	manifest := `{"name":"Nested Zip ??","version":"3.0.0","description":"nested zip"}`
	if err := os.WriteFile(zipPath, buildTestZipArchiveWithRootDir(t, "nested-root", manifest), 0o644); err != nil {
		t.Fatalf("???? ZIP ??: %v", err)
	}

	item, err := mgr.ImportExtensionCRX(zipPath)
	if err != nil {
		t.Fatalf("???? ZIP ????: %v", err)
	}
	if item.SourceType != "zip" {
		t.Fatalf("SourceType ?? zip???=%s", item.SourceType)
	}
	if item.Name != "Nested Zip ??" || item.Version != "3.0.0" {
		t.Fatalf("?? ZIP ????????: %+v", item)
	}
	manifestPath := filepath.Join(item.UnpackedPath, "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("?? ZIP ???? manifest.json ???: %v", err)
	}
}

func TestImportExtensionFromURLDownloadsAndImports(t *testing.T) {
	t.Parallel()
	db := newExtensionTestDB(t)
	mgr := NewManager(&config.Config{}, t.TempDir())
	mgr.ExtensionDAO = NewSQLiteExtensionDAO(db.GetConn())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-chrome-extension")
		_, _ = w.Write(buildTestCRX(t, `{"name":"????","version":"9.9.9"}`))
	}))
	defer server.Close()

	item, err := mgr.ImportExtensionURL(server.URL)
	if err != nil {
		t.Fatalf("? URL ??????: %v", err)
	}
	if item.Name != "????" || item.SourceType != "crx" {
		t.Fatalf("?????????: %+v", item)
	}
	if item.SourcePath != server.URL {
		t.Fatalf("SourcePath ??????????=%s", item.SourcePath)
	}
}

func TestBuildChromeWebStoreDownloadURLIncludesExtensionID(t *testing.T) {
	t.Parallel()
	url := BuildChromeWebStoreDownloadURL("aapbdbdomjkkjkaonfhkkikfgjllcleb")
	if !strings.Contains(url, "clients2.google.com/service/update2/crx") {
		t.Fatalf("???????: %s", url)
	}
	if !strings.Contains(url, "aapbdbdomjkkjkaonfhkkikfgjllcleb") {
		t.Fatalf("????????? ID: %s", url)
	}
}

func TestImportExtensionDirResolvesLocaleMessages(t *testing.T) {
	t.Parallel()
	db := newExtensionTestDB(t)
	mgr := NewManager(&config.Config{}, t.TempDir())
	mgr.ExtensionDAO = NewSQLiteExtensionDAO(db.GetConn())

	dir := filepath.Join(t.TempDir(), "ext")
	if err := os.MkdirAll(filepath.Join(dir, "_locales", "en"), 0o755); err != nil {
		t.Fatalf("mkdir locales: %v", err)
	}
	manifest := `{"name":"__MSG_name__","description":"__MSG_desc__","version":"1.0.0","default_locale":"en"}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	messages := `{"name":{"message":"Cookie Editor"},"desc":{"message":"Edit cookies"}}`
	if err := os.WriteFile(filepath.Join(dir, "_locales", "en", "messages.json"), []byte(messages), 0o644); err != nil {
		t.Fatalf("write messages: %v", err)
	}

	item, err := mgr.ImportExtensionDir(dir)
	if err != nil {
		t.Fatalf("import dir failed: %v", err)
	}
	if item.Name != "Cookie Editor" {
		t.Fatalf("expected localized name, got=%q", item.Name)
	}
	if item.Description != "Edit cookies" {
		t.Fatalf("expected localized description, got=%q", item.Description)
	}
}

func TestListExtensionsBackfillsLocaleMessages(t *testing.T) {
	t.Parallel()
	db := newExtensionTestDB(t)
	mgr := NewManager(&config.Config{}, t.TempDir())
	dao := NewSQLiteExtensionDAO(db.GetConn())
	mgr.ExtensionDAO = dao

	stored := filepath.Join(t.TempDir(), "stored-ext")
	if err := os.MkdirAll(filepath.Join(stored, "_locales", "en"), 0o755); err != nil {
		t.Fatalf("mkdir locales: %v", err)
	}
	manifest := `{"name":"__MSG_name__","description":"__MSG_desc__","version":"1.0.0","default_locale":"en"}`
	if err := os.WriteFile(filepath.Join(stored, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	messages := `{"name":{"message":"Cookie Editor"},"desc":{"message":"Edit cookies"}}`
	if err := os.WriteFile(filepath.Join(stored, "_locales", "en", "messages.json"), []byte(messages), 0o644); err != nil {
		t.Fatalf("write messages: %v", err)
	}
	if err := dao.Upsert(&Extension{ExtensionId: "ext-old", Name: "__MSG_name__", Description: "__MSG_desc__", SourceType: "crx", SourcePath: "x", UnpackedPath: stored, Version: "1.0.0", EnabledByDefault: true}); err != nil {
		t.Fatalf("seed extension: %v", err)
	}
	items := mgr.ListExtensions()
	if len(items) != 1 || items[0].Name != "Cookie Editor" || items[0].Description != "Edit cookies" {
		t.Fatalf("expected localized extension from list, got=%+v", items)
	}
}

func TestExtensionLoadPathsIncludesGlobalCRXExtensionsWithoutProfileBindings(t *testing.T) {
	t.Parallel()
	db := newExtensionTestDB(t)
	mgr := NewManager(&config.Config{}, t.TempDir())
	mgr.ExtensionDAO = NewSQLiteExtensionDAO(db.GetConn())
	mgr.ProfileDAO = NewSQLiteProfileDAO(db.GetConn())
	mgr.ProfileExtensionDAO = NewSQLiteProfileExtensionDAO(db.GetConn())
	if err := mgr.ProfileDAO.Upsert(&Profile{ProfileId: "profile-1", ProfileName: "P1", UserDataDir: "profile-1", CoreId: "core-1", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatalf("seed profile: %v", err)
	}

	globalExt := &Extension{ExtensionId: "ext-global", Name: "Global CRX", SourceType: "crx", SourcePath: "demo.crx", UnpackedPath: filepath.Join(t.TempDir(), "global"), Version: "1.0.0", EnabledByDefault: true}
	localExt := &Extension{ExtensionId: "ext-local", Name: "Local Dir", SourceType: "unpacked", SourcePath: "demo-dir", UnpackedPath: filepath.Join(t.TempDir(), "local"), Version: "1.0.0", EnabledByDefault: false}
	if err := os.MkdirAll(globalExt.UnpackedPath, 0o755); err != nil {
		t.Fatalf("mkdir global: %v", err)
	}
	if err := os.MkdirAll(localExt.UnpackedPath, 0o755); err != nil {
		t.Fatalf("mkdir local: %v", err)
	}
	if err := mgr.ExtensionDAO.Upsert(globalExt); err != nil {
		t.Fatalf("seed global ext: %v", err)
	}
	if err := mgr.ExtensionDAO.Upsert(localExt); err != nil {
		t.Fatalf("seed local ext: %v", err)
	}
	if err := mgr.ProfileExtensionDAO.ReplaceProfileBindings("profile-1", []*ProfileExtensionBinding{{BindingId: "bind-local", ProfileId: "profile-1", ExtensionId: "ext-local", Enabled: true, SortOrder: 1, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}}); err != nil {
		t.Fatalf("seed bindings: %v", err)
	}

	paths, err := mgr.ExtensionLoadPaths("profile-1")
	if err != nil {
		t.Fatalf("ExtensionLoadPaths failed: %v", err)
	}
	want := map[string]struct{}{globalExt.UnpackedPath: {}, localExt.UnpackedPath: {}}
	if len(paths) != len(want) {
		t.Fatalf("unexpected path count: got=%v want=%v", paths, want)
	}
	for _, item := range paths {
		if _, ok := want[item]; !ok {
			t.Fatalf("unexpected path: %s (all=%v)", item, paths)
		}
	}
}

func TestImportExtensionDirDefaultsToLocalBindingScope(t *testing.T) {
	t.Parallel()
	db := newExtensionTestDB(t)
	mgr := NewManager(&config.Config{}, t.TempDir())
	mgr.ExtensionDAO = NewSQLiteExtensionDAO(db.GetConn())

	dir := filepath.Join(t.TempDir(), "ext-local-default")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	manifest := `{"name":"Local Only","description":"local","version":"1.0.0"}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	item, err := mgr.ImportExtensionDir(dir)
	if err != nil {
		t.Fatalf("ImportExtensionDir failed: %v", err)
	}
	if item.EnabledByDefault {
		t.Fatalf("expected unpacked extension to default to local scope, got enabledByDefault=true")
	}
}

func TestSetExtensionEnabledByDefaultAllowsUnpackedExtension(t *testing.T) {
	t.Parallel()
	db := newExtensionTestDB(t)
	mgr := NewManager(&config.Config{}, t.TempDir())
	dao := NewSQLiteExtensionDAO(db.GetConn())
	mgr.ExtensionDAO = dao
	item := &Extension{ExtensionId: "ext-unpacked", Name: "Local", SourceType: "unpacked", SourcePath: "dir", UnpackedPath: filepath.Join(t.TempDir(), "local-unpacked"), Version: "1.0.0", EnabledByDefault: false}
	if err := os.MkdirAll(item.UnpackedPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := dao.Upsert(item); err != nil {
		t.Fatalf("seed extension: %v", err)
	}

	updated, err := mgr.SetExtensionEnabledByDefault(item.ExtensionId, true)
	if err != nil {
		t.Fatalf("SetExtensionEnabledByDefault failed: %v", err)
	}
	if !updated.EnabledByDefault {
		t.Fatalf("expected enabledByDefault=true")
	}
	stored, err := dao.GetById(item.ExtensionId)
	if err != nil {
		t.Fatalf("GetById failed: %v", err)
	}
	if !stored.EnabledByDefault {
		t.Fatalf("expected stored enabledByDefault=true")
	}
}

func TestRefreshExtensionPreservesExtensionID(t *testing.T) {
	t.Parallel()
	db := newExtensionTestDB(t)
	mgr := NewManager(&config.Config{}, t.TempDir())
	dao := NewSQLiteExtensionDAO(db.GetConn())
	mgr.ExtensionDAO = dao

	zipPath := filepath.Join(t.TempDir(), "refresh.zip")
	manifest1 := `{"name":"Refresh Demo","version":"1.0.0","description":"first"}`
	if err := os.WriteFile(zipPath, buildTestZipArchiveWithRootDir(t, "refresh-root", manifest1), 0o644); err != nil {
		t.Fatalf("write zip v1: %v", err)
	}
	item, err := mgr.ImportExtensionCRX(zipPath)
	if err != nil {
		t.Fatalf("ImportExtensionCRX failed: %v", err)
	}
	manifest2 := `{"name":"Refresh Demo","version":"2.0.0","description":"second"}`
	if err := os.WriteFile(zipPath, buildTestZipArchiveWithRootDir(t, "refresh-root", manifest2), 0o644); err != nil {
		t.Fatalf("write zip v2: %v", err)
	}

	refreshed, err := mgr.RefreshExtension(item.ExtensionId)
	if err != nil {
		t.Fatalf("RefreshExtension failed: %v", err)
	}
	if refreshed.ExtensionId != item.ExtensionId {
		t.Fatalf("expected same extension id, got=%s want=%s", refreshed.ExtensionId, item.ExtensionId)
	}
	if refreshed.Version != "2.0.0" {
		t.Fatalf("expected refreshed version 2.0.0, got=%s", refreshed.Version)
	}
}
