package backend

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"strings"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/proxy"
	"github.com/google/uuid"
)

func (a *App) BrowserSubscriptionList() []browser.SubscriptionSource {
	if a.browserMgr.SubscriptionDAO == nil {
		return nil
	}
	list, err := a.browserMgr.SubscriptionDAO.List()
	if err != nil {
		return nil
	}
	return list
}

func (a *App) BrowserSubscriptionSave(input browser.SubscriptionSource) (browser.SubscriptionSource, error) {
	if a.browserMgr.SubscriptionDAO == nil {
		return browser.SubscriptionSource{}, fmt.Errorf("subscription dao unavailable")
	}
	input.Name = strings.TrimSpace(input.Name)
	input.URL = strings.TrimSpace(input.URL)
	var existing *browser.SubscriptionSource
	if input.SourceID != "" {
		existing, _ = a.browserMgr.SubscriptionDAO.Get(input.SourceID)
	}
	if input.SourceID == "" {
		input.SourceID = uuid.New().String()
	}
	if input.RefreshIntervalMinutes <= 0 {
		input.RefreshIntervalMinutes = 60
	}
	if existing == nil && strings.TrimSpace(input.ImportMode) == "" {
		input.ImportMode = "selected"
	}
	input = mergeSubscriptionSource(input, existing)
	if err := a.browserMgr.SubscriptionDAO.Upsert(input); err != nil {
		return browser.SubscriptionSource{}, err
	}
	if err := a.syncSubscriptionSelections(input); err != nil {
		return browser.SubscriptionSource{}, err
	}
	return input, nil
}

func (a *App) BrowserSubscriptionDelete(sourceID string) error {
	if a.browserMgr.SubscriptionDAO == nil {
		return fmt.Errorf("subscription dao unavailable")
	}
	if a.browserMgr.ProxyDAO != nil {
		if err := a.browserMgr.ProxyDAO.DeleteBySource(sourceID); err != nil {
			return err
		}
	}
	if a.browserMgr.SubscriptionNodeDAO != nil {
		if err := a.browserMgr.SubscriptionNodeDAO.DeleteBySource(sourceID); err != nil {
			return err
		}
	}
	return a.browserMgr.SubscriptionDAO.Delete(sourceID)
}

func (a *App) BrowserSubscriptionRefresh(sourceID string) error {
	log := logger.New("Subscription")
	if a.browserMgr.SubscriptionDAO == nil || a.browserMgr.ProxyDAO == nil {
		return fmt.Errorf("subscription refresh dependencies unavailable")
	}
	source, err := a.browserMgr.SubscriptionDAO.Get(sourceID)
	if err != nil {
		return err
	}
	log.Info("开始刷新订阅", logger.F("source_id", source.SourceID), logger.F("name", source.Name), logger.F("url", source.URL))
	preview, err := a.BrowserProxyFetchClashByURL(source.URL)
	if err != nil {
		log.Error("订阅抓取失败", logger.F("source_id", source.SourceID), logger.F("error", err.Error()))
		return persistSubscriptionRefreshFailure(a.browserMgr.SubscriptionDAO, source, err)
	}
	rawContent, _ := preview["content"].(string)
	doc, err := proxy.ParseSubscriptionDocument([]byte(rawContent), source.SourceID, source.URL)
	if err != nil {
		log.Error("订阅解析失败", logger.F("source_id", source.SourceID), logger.F("error", err.Error()))
		return persistSubscriptionRefreshFailure(a.browserMgr.SubscriptionDAO, source, err)
	}
	log.Info("订阅解析完成", logger.F("source_id", source.SourceID), logger.F("node_count", len(doc.Nodes)), logger.F("group_count", len(doc.Groups)))
	if err := a.refreshSubscriptionDocument(source, doc, rawContent); err != nil {
		log.Error("订阅同步失败", logger.F("source_id", source.SourceID), logger.F("error", err.Error()))
		return persistSubscriptionRefreshFailure(a.browserMgr.SubscriptionDAO, source, err)
	}
	log.Info("订阅刷新成功", logger.F("source_id", source.SourceID), logger.F("name", source.Name), logger.F("node_count", len(doc.Nodes)))
	return nil
}
