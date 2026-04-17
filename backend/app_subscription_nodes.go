package backend

import "ant-chrome/backend/internal/browser"

func (a *App) BrowserSubscriptionNodeList(sourceID string) []browser.SubscriptionNode {
	if a.browserMgr == nil || a.browserMgr.SubscriptionNodeDAO == nil {
		return nil
	}
	list, err := a.browserMgr.SubscriptionNodeDAO.ListBySource(sourceID)
	if err != nil {
		return nil
	}
	return list
}
