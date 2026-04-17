package browser

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type SubscriptionDAO interface {
	List() ([]SubscriptionSource, error)
	Get(sourceID string) (*SubscriptionSource, error)
	Upsert(source SubscriptionSource) error
	Delete(sourceID string) error
}

type SQLiteSubscriptionDAO struct{ db *sql.DB }

func NewSQLiteSubscriptionDAO(db *sql.DB) *SQLiteSubscriptionDAO {
	return &SQLiteSubscriptionDAO{db: db}
}

func (d *SQLiteSubscriptionDAO) List() ([]SubscriptionSource, error) {
	rows, err := d.db.Query(`SELECT source_id, name, url, enabled, refresh_interval_minutes, last_refresh_at, last_refresh_status, last_error, traffic_used, traffic_total, expire_at, raw_content_hash, proxy_groups_json, selected_proxy_groups_json, import_mode, selected_node_keys_json, import_stats_json FROM subscription_sources ORDER BY name ASC, source_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询订阅源失败: %w", err)
	}
	defer rows.Close()
	var list []SubscriptionSource
	for rows.Next() {
		item, err := scanSubscriptionSource(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *item)
	}
	return list, rows.Err()
}

func (d *SQLiteSubscriptionDAO) Get(sourceID string) (*SubscriptionSource, error) {
	row := d.db.QueryRow(`SELECT source_id, name, url, enabled, refresh_interval_minutes, last_refresh_at, last_refresh_status, last_error, traffic_used, traffic_total, expire_at, raw_content_hash, proxy_groups_json, selected_proxy_groups_json, import_mode, selected_node_keys_json, import_stats_json FROM subscription_sources WHERE source_id = ?`, sourceID)
	item, err := scanSubscriptionSource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("订阅源不存在: %s", sourceID)
	}
	return item, err
}

func (d *SQLiteSubscriptionDAO) Upsert(source SubscriptionSource) error {
	_, err := d.db.Exec(`INSERT INTO subscription_sources (source_id, name, url, enabled, refresh_interval_minutes, last_refresh_at, last_refresh_status, last_error, traffic_used, traffic_total, expire_at, raw_content_hash, proxy_groups_json, selected_proxy_groups_json, import_mode, selected_node_keys_json, import_stats_json, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(source_id) DO UPDATE SET name=excluded.name, url=excluded.url, enabled=excluded.enabled, refresh_interval_minutes=excluded.refresh_interval_minutes, last_refresh_at=excluded.last_refresh_at, last_refresh_status=excluded.last_refresh_status, last_error=excluded.last_error, traffic_used=excluded.traffic_used, traffic_total=excluded.traffic_total, expire_at=excluded.expire_at, raw_content_hash=excluded.raw_content_hash, proxy_groups_json=excluded.proxy_groups_json, selected_proxy_groups_json=excluded.selected_proxy_groups_json, import_mode=excluded.import_mode, selected_node_keys_json=excluded.selected_node_keys_json, import_stats_json=excluded.import_stats_json, updated_at=excluded.updated_at`, source.SourceID, source.Name, source.URL, boolToIntSubscription(source.Enabled), source.RefreshIntervalMinutes, source.LastRefreshAt, source.LastRefreshStatus, source.LastError, source.TrafficUsed, source.TrafficTotal, source.ExpireAt, source.RawContentHash, source.ProxyGroupsJSON, source.SelectedProxyGroupsJSON, source.ImportMode, source.SelectedNodeKeysJSON, source.ImportStatsJSON, time.Now().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("保存订阅源失败: %w", err)
	}
	return nil
}

func (d *SQLiteSubscriptionDAO) Delete(sourceID string) error {
	_, err := d.db.Exec(`DELETE FROM subscription_sources WHERE source_id = ?`, sourceID)
	if err != nil {
		return fmt.Errorf("删除订阅源失败: %w", err)
	}
	return nil
}

func scanSubscriptionSource(s scanner) (*SubscriptionSource, error) {
	var item SubscriptionSource
	var enabled int
	if err := s.Scan(&item.SourceID, &item.Name, &item.URL, &enabled, &item.RefreshIntervalMinutes, &item.LastRefreshAt, &item.LastRefreshStatus, &item.LastError, &item.TrafficUsed, &item.TrafficTotal, &item.ExpireAt, &item.RawContentHash, &item.ProxyGroupsJSON, &item.SelectedProxyGroupsJSON, &item.ImportMode, &item.SelectedNodeKeysJSON, &item.ImportStatsJSON); err != nil {
		return nil, err
	}
	item.Enabled = enabled == 1
	return &item, nil
}

func boolToIntSubscription(value bool) int {
	if value {
		return 1
	}
	return 0
}
