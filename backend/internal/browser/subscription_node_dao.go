package browser

import (
	"database/sql"
	"fmt"
)

type SubscriptionNodeDAO interface {
	ListBySource(sourceID string) ([]SubscriptionNode, error)
	ReplaceBySource(sourceID string, nodes []SubscriptionNode) error
	DeleteBySource(sourceID string) error
}

type SQLiteSubscriptionNodeDAO struct{ db *sql.DB }

func NewSQLiteSubscriptionNodeDAO(db *sql.DB) *SQLiteSubscriptionNodeDAO {
	return &SQLiteSubscriptionNodeDAO{db: db}
}

func (d *SQLiteSubscriptionNodeDAO) ListBySource(sourceID string) ([]SubscriptionNode, error) {
	rows, err := d.db.Query(`SELECT node_key, source_id, node_name, protocol, server, port, display_group, chain_mode, upstream_alias, node_json FROM subscription_nodes WHERE source_id = ? ORDER BY display_group ASC, node_name ASC, node_key ASC`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("??????????: %w", err)
	}
	defer rows.Close()
	var list []SubscriptionNode
	for rows.Next() {
		item, err := scanSubscriptionNode(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *item)
	}
	return list, rows.Err()
}

func (d *SQLiteSubscriptionNodeDAO) ReplaceBySource(sourceID string, nodes []SubscriptionNode) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("??????????: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM subscription_nodes WHERE source_id = ?`, sourceID); err != nil {
		return fmt.Errorf("??????????: %w", err)
	}
	for _, node := range nodes {
		if _, err := tx.Exec(`INSERT INTO subscription_nodes (node_key, source_id, node_name, protocol, server, port, display_group, chain_mode, upstream_alias, node_json, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`, node.NodeKey, node.SourceID, node.NodeName, node.Protocol, node.Server, node.Port, node.DisplayGroup, node.ChainMode, node.UpstreamAlias, node.NodeJSON); err != nil {
			return fmt.Errorf("??????????: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("??????????: %w", err)
	}
	return nil
}

func (d *SQLiteSubscriptionNodeDAO) DeleteBySource(sourceID string) error {
	if _, err := d.db.Exec(`DELETE FROM subscription_nodes WHERE source_id = ?`, sourceID); err != nil {
		return fmt.Errorf("??????????: %w", err)
	}
	return nil
}

func scanSubscriptionNode(s scanner) (*SubscriptionNode, error) {
	var item SubscriptionNode
	if err := s.Scan(&item.NodeKey, &item.SourceID, &item.NodeName, &item.Protocol, &item.Server, &item.Port, &item.DisplayGroup, &item.ChainMode, &item.UpstreamAlias, &item.NodeJSON); err != nil {
		return nil, err
	}
	return &item, nil
}
