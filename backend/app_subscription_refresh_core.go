package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/proxy"
	"crypto/sha1"
	"fmt"
	"strings"
	"time"
)

func (a *App) refreshSubscriptionDocument(source *browser.SubscriptionSource, doc proxy.SubscriptionDocument, rawContent string) error {
	log := logger.New("Subscription")
	log.Info("开始同步订阅文档", logger.F("source_id", source.SourceID), logger.F("raw_len", len(rawContent)), logger.F("node_count", len(doc.Nodes)), logger.F("group_count", len(doc.Groups)))

	list, err := a.browserMgr.ProxyDAO.List()
	if err != nil {
		return err
	}
	var existing []browser.Proxy
	for _, item := range list {
		if strings.EqualFold(strings.TrimSpace(item.SourceID), strings.TrimSpace(source.SourceID)) {
			existing = append(existing, item)
		}
	}

	doc.Nodes = mergeProxyRuntimeState(doc.Nodes, existing)
	source.ProxyGroupsJSON = marshalProxyGroups(doc.Groups)
	selected := normalizeSelectedProxyGroups(doc.Groups, parseSelectedProxyGroups(source.SelectedProxyGroupsJSON), doc.Nodes)
	effectiveSelected := buildEffectiveSelectedProxyGroups(doc.Groups, selected, doc.Nodes)
	source.SelectedProxyGroupsJSON = marshalSelectedProxyGroups(selected)
	resolvedNodes := applySelectorChoices(doc.Nodes, doc.Groups, effectiveSelected)
	catalog := buildSubscriptionCatalog(resolvedNodes)
	log.Info("订阅目录已生成", logger.F("source_id", source.SourceID), logger.F("catalog_count", len(catalog)))

	if a.browserMgr.SubscriptionNodeDAO != nil {
		log.Info("开始写入订阅目录节点", logger.F("source_id", source.SourceID), logger.F("count", len(catalog)))
		if err := a.browserMgr.SubscriptionNodeDAO.ReplaceBySource(source.SourceID, catalog); err != nil {
			return err
		}
	}

	selectedNodeKeys := parseSelectedNodeKeySet(source.SelectedNodeKeysJSON)
	importMode := strings.TrimSpace(source.ImportMode)
	if importMode == "" {
		if len(existing) > 0 {
			importMode = "selected"
			if len(selectedNodeKeys) == 0 {
				for _, item := range existing {
					if key := strings.TrimSpace(item.ProxyId); key != "" {
						selectedNodeKeys[key] = struct{}{}
					}
				}
				if len(selectedNodeKeys) > 0 {
					source.SelectedNodeKeysJSON = marshalSelectedNodeKeys(selectedNodeKeys)
				}
			}
		} else {
			importMode = "all"
		}
	}

	materialized := resolvedNodes
	if strings.EqualFold(importMode, "selected") {
		materialized = materializeSelectedNodes(resolvedNodes, selectedNodeKeys)
	}

	log.Info("开始清理旧代理", logger.F("source_id", source.SourceID), logger.F("existing_count", len(existing)))
	if err := a.browserMgr.ProxyDAO.DeleteBySource(source.SourceID); err != nil {
		return err
	}

	log.Info("开始写入导入代理", logger.F("source_id", source.SourceID), logger.F("import_mode", importMode), logger.F("materialized_count", len(materialized)))
	for _, node := range materialized {
		if err := a.browserMgr.ProxyDAO.Upsert(node); err != nil {
			return err
		}
	}

	source.ImportMode = importMode
	source.ImportStatsJSON = marshalImportStatsJSON(catalog, selectedNodeKeys, materialized)
	source.LastRefreshAt = time.Now().Format(time.RFC3339)
	source.LastRefreshStatus = "ok"
	source.LastError = ""
	source.RawContentHash = fmt.Sprintf("%x", sha1.Sum([]byte(rawContent)))

	log.Info("开始写回订阅状态", logger.F("source_id", source.SourceID), logger.F("last_refresh_status", source.LastRefreshStatus), logger.F("import_stats", source.ImportStatsJSON))
	if err := a.browserMgr.SubscriptionDAO.Upsert(*source); err != nil {
		return err
	}
	log.Info("订阅文档同步完成", logger.F("source_id", source.SourceID), logger.F("imported_count", len(materialized)))
	return nil
}
