package browser

import (
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/database"
)

func newExtensionTestDB(t *testing.T) *database.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "extensions.db")
	db, err := database.NewDB(dbPath)
	if err != nil {
		t.Fatalf("创建测试数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("执行数据库迁移失败: %v", err)
	}
	return db
}

func TestSQLiteExtensionDAO_CRUD(t *testing.T) {
	t.Parallel()
	db := newExtensionTestDB(t)
	dao := NewSQLiteExtensionDAO(db.GetConn())

	ext := &Extension{
		ExtensionId:      "ext-1",
		Name:             "广告拦截",
		SourceType:       "unpacked",
		SourcePath:       `D:\\ext\\adblock`,
		UnpackedPath:     `D:\\repo\\extensions\\ext-1`,
		Version:          "1.0.0",
		Description:      "用于测试",
		Permissions:      []string{"storage", "tabs"},
		HostPermissions:  []string{"https://*/*"},
		OptionsPage:      "options.html",
		IconPath:         "icons/128.png",
		EnabledByDefault: true,
	}
	if err := dao.Upsert(ext); err != nil {
		t.Fatalf("首次保存扩展失败: %v", err)
	}

	list, err := dao.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("查询扩展列表失败: len=%d err=%v", len(list), err)
	}
	if list[0].Name != "广告拦截" || list[0].Version != "1.0.0" {
		t.Fatalf("扩展字段不正确: %+v", list[0])
	}

	ext.Name = "广告拦截 Plus"
	ext.EnabledByDefault = false
	if err := dao.Upsert(ext); err != nil {
		t.Fatalf("更新扩展失败: %v", err)
	}
	got, err := dao.GetById("ext-1")
	if err != nil {
		t.Fatalf("按 ID 查询扩展失败: %v", err)
	}
	if got.Name != "广告拦截 Plus" || got.EnabledByDefault {
		t.Fatalf("扩展更新未生效: %+v", got)
	}

	if err := dao.Delete("ext-1"); err != nil {
		t.Fatalf("删除扩展失败: %v", err)
	}
	list, err = dao.List()
	if err != nil || len(list) != 0 {
		t.Fatalf("删除后扩展列表异常: len=%d err=%v", len(list), err)
	}
}

func TestSQLiteProfileExtensionDAO_ReplaceProfileBindings(t *testing.T) {
	t.Parallel()
	db := newExtensionTestDB(t)
	extDAO := NewSQLiteExtensionDAO(db.GetConn())
	bindingDAO := NewSQLiteProfileExtensionDAO(db.GetConn())

	for _, ext := range []*Extension{
		{ExtensionId: "ext-a", Name: "A", SourceType: "unpacked", SourcePath: "a", UnpackedPath: "a", Version: "1.0.0"},
		{ExtensionId: "ext-b", Name: "B", SourceType: "unpacked", SourcePath: "b", UnpackedPath: "b", Version: "1.0.0"},
	} {
		if err := extDAO.Upsert(ext); err != nil {
			t.Fatalf("准备扩展数据失败: %v", err)
		}
	}

	if _, err := db.GetConn().Exec(`INSERT INTO browser_profiles (profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config, launch_args, tags, keywords, group_id, created_at, updated_at) VALUES ('profile-1', '????', '', '', '[]', '', '', '[]', '[]', '[]', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("????????: %v", err)
	}

	bindings := []*ProfileExtensionBinding{
		{BindingId: "b-1", ProfileId: "profile-1", ExtensionId: "ext-a", Enabled: true, SortOrder: 10},
		{BindingId: "b-2", ProfileId: "profile-1", ExtensionId: "ext-b", Enabled: false, SortOrder: 20},
	}
	if err := bindingDAO.ReplaceProfileBindings("profile-1", bindings); err != nil {
		t.Fatalf("首次替换绑定失败: %v", err)
	}
	list, err := bindingDAO.ListByProfile("profile-1")
	if err != nil || len(list) != 2 {
		t.Fatalf("查询实例扩展绑定失败: len=%d err=%v", len(list), err)
	}
	if list[0].ExtensionId != "ext-a" || list[1].ExtensionId != "ext-b" {
		t.Fatalf("绑定排序异常: %+v", list)
	}

	replaced := []*ProfileExtensionBinding{
		{BindingId: "b-3", ProfileId: "profile-1", ExtensionId: "ext-b", Enabled: true, SortOrder: 5},
	}
	if err := bindingDAO.ReplaceProfileBindings("profile-1", replaced); err != nil {
		t.Fatalf("二次替换绑定失败: %v", err)
	}
	list, err = bindingDAO.ListByProfile("profile-1")
	if err != nil || len(list) != 1 {
		t.Fatalf("替换后绑定数量异常: len=%d err=%v", len(list), err)
	}
	if list[0].ExtensionId != "ext-b" || !list[0].Enabled || list[0].SortOrder != 5 {
		t.Fatalf("替换后的绑定不正确: %+v", list[0])
	}
}
