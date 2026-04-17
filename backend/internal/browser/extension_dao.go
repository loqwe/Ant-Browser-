package browser

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type ExtensionDAO interface {
	List() ([]*Extension, error)
	GetById(extensionId string) (*Extension, error)
	Upsert(extension *Extension) error
	Delete(extensionId string) error
}

type ProfileExtensionDAO interface {
	ListByProfile(profileId string) ([]*ProfileExtensionBinding, error)
	ReplaceProfileBindings(profileId string, bindings []*ProfileExtensionBinding) error
}

type SQLiteExtensionDAO struct{ db *sql.DB }

type SQLiteProfileExtensionDAO struct{ db *sql.DB }

func NewSQLiteExtensionDAO(db *sql.DB) *SQLiteExtensionDAO { return &SQLiteExtensionDAO{db: db} }

func NewSQLiteProfileExtensionDAO(db *sql.DB) *SQLiteProfileExtensionDAO {
	return &SQLiteProfileExtensionDAO{db: db}
}

func (d *SQLiteExtensionDAO) List() ([]*Extension, error) {
	rows, err := d.db.Query(`SELECT extension_id,name,source_type,source_path,unpacked_path,version,description,permissions,host_permissions,options_page,icon_path,enabled_by_default,created_at,updated_at FROM browser_extensions ORDER BY created_at ASC`)
	if err != nil { return nil, fmt.Errorf("查询扩展列表失败: %w", err) }
	defer rows.Close()
	var items []*Extension
	for rows.Next() { item, err := scanExtension(rows); if err != nil { return nil, err }; items = append(items, item) }
	return items, rows.Err()
}

func (d *SQLiteExtensionDAO) GetById(extensionId string) (*Extension, error) {
	row := d.db.QueryRow(`SELECT extension_id,name,source_type,source_path,unpacked_path,version,description,permissions,host_permissions,options_page,icon_path,enabled_by_default,created_at,updated_at FROM browser_extensions WHERE extension_id = ?`, extensionId)
	return scanExtension(row)
}

func (d *SQLiteExtensionDAO) Upsert(extension *Extension) error {
	now := time.Now().Format(time.RFC3339)
	if extension.CreatedAt == "" { extension.CreatedAt = now }
	extension.UpdatedAt = now
	permissions, _ := json.Marshal(extension.Permissions)
	hostPermissions, _ := json.Marshal(extension.HostPermissions)
	_, err := d.db.Exec(`INSERT INTO browser_extensions (extension_id,name,source_type,source_path,unpacked_path,version,description,permissions,host_permissions,options_page,icon_path,enabled_by_default,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(extension_id) DO UPDATE SET name=excluded.name,source_type=excluded.source_type,source_path=excluded.source_path,unpacked_path=excluded.unpacked_path,version=excluded.version,description=excluded.description,permissions=excluded.permissions,host_permissions=excluded.host_permissions,options_page=excluded.options_page,icon_path=excluded.icon_path,enabled_by_default=excluded.enabled_by_default,updated_at=excluded.updated_at`, extension.ExtensionId, extension.Name, extension.SourceType, extension.SourcePath, extension.UnpackedPath, extension.Version, extension.Description, string(permissions), string(hostPermissions), extension.OptionsPage, extension.IconPath, boolToInt(extension.EnabledByDefault), extension.CreatedAt, extension.UpdatedAt)
	if err != nil { return fmt.Errorf("保存扩展失败: %w", err) }
	return nil
}

func (d *SQLiteExtensionDAO) Delete(extensionId string) error {
	_, err := d.db.Exec(`DELETE FROM browser_extensions WHERE extension_id = ?`, extensionId)
	if err != nil { return fmt.Errorf("删除扩展失败: %w", err) }
	return nil
}

func (d *SQLiteProfileExtensionDAO) ListByProfile(profileId string) ([]*ProfileExtensionBinding, error) {
	rows, err := d.db.Query(`SELECT binding_id,profile_id,extension_id,enabled,sort_order,created_at,updated_at FROM browser_profile_extensions WHERE profile_id = ? ORDER BY sort_order ASC, created_at ASC`, profileId)
	if err != nil { return nil, fmt.Errorf("查询实例扩展绑定失败: %w", err) }
	defer rows.Close()
	var items []*ProfileExtensionBinding
	for rows.Next() { item, err := scanProfileExtensionBinding(rows); if err != nil { return nil, err }; items = append(items, item) }
	return items, rows.Err()
}

func (d *SQLiteProfileExtensionDAO) ReplaceProfileBindings(profileId string, bindings []*ProfileExtensionBinding) error {
	tx, err := d.db.Begin()
	if err != nil { return fmt.Errorf("开启绑定事务失败: %w", err) }
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM browser_profile_extensions WHERE profile_id = ?`, profileId); err != nil { return fmt.Errorf("清理旧绑定失败: %w", err) }
	for _, item := range bindings {
		now := time.Now().Format(time.RFC3339)
		if item.CreatedAt == "" { item.CreatedAt = now }
		item.UpdatedAt = now
		if _, err := tx.Exec(`INSERT INTO browser_profile_extensions (binding_id,profile_id,extension_id,enabled,sort_order,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`, item.BindingId, profileId, item.ExtensionId, boolToInt(item.Enabled), item.SortOrder, item.CreatedAt, item.UpdatedAt); err != nil { return fmt.Errorf("保存实例扩展绑定失败: %w", err) }
	}
	return tx.Commit()
}

func scanExtension(scanner interface{ Scan(...any) error }) (*Extension, error) {
	var item Extension
	var permissions, hostPermissions string
	var enabled int
	if err := scanner.Scan(&item.ExtensionId, &item.Name, &item.SourceType, &item.SourcePath, &item.UnpackedPath, &item.Version, &item.Description, &permissions, &hostPermissions, &item.OptionsPage, &item.IconPath, &enabled, &item.CreatedAt, &item.UpdatedAt); err != nil { return nil, err }
	_ = json.Unmarshal([]byte(permissions), &item.Permissions)
	_ = json.Unmarshal([]byte(hostPermissions), &item.HostPermissions)
	item.EnabledByDefault = enabled == 1
	return &item, nil
}

func scanProfileExtensionBinding(scanner interface{ Scan(...any) error }) (*ProfileExtensionBinding, error) {
	var item ProfileExtensionBinding
	var enabled int
	if err := scanner.Scan(&item.BindingId, &item.ProfileId, &item.ExtensionId, &enabled, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil { return nil, err }
	item.Enabled = enabled == 1
	return &item, nil
}

func boolToInt(v bool) int {
	if v { return 1 }
	return 0
}
