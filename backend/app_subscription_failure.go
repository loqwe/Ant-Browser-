package backend

import (
	"ant-chrome/backend/internal/logger"
	"errors"
	"fmt"
	"strings"
	"time"

	"ant-chrome/backend/internal/browser"
)

func persistSubscriptionRefreshFailure(dao browser.SubscriptionDAO, source *browser.SubscriptionSource, refreshErr error) error {
	log := logger.New("Subscription")
	message := strings.TrimSpace(refreshErr.Error())
	if source == nil || dao == nil {
		return errors.New(message)
	}
	source.LastRefreshAt = time.Now().Format(time.RFC3339)
	source.LastRefreshStatus = "刷新失败"
	source.LastError = message
	if shouldPauseSubscriptionRefresh(message) {
		source.Enabled = false
		source.LastRefreshStatus = "已暂停"
		if !strings.Contains(message, "已自动暂停订阅刷新") {
			message = message + "\n已自动暂停订阅刷新。"
		}
		source.LastError = message
	}
	log.Error("订阅刷新失败已写回状态", logger.F("source_id", source.SourceID), logger.F("status", source.LastRefreshStatus), logger.F("error", source.LastError))
	if err := dao.Upsert(*source); err != nil {
		return fmt.Errorf("%s\n状态写回失败: %v", message, err)
	}
	return errors.New(message)
}

func shouldPauseSubscriptionRefresh(message string) bool {
	message = strings.TrimSpace(message)
	return strings.Contains(message, "账号已被封禁") || strings.Contains(message, "封禁截止") || strings.Contains(message, "速率限制连续触发")
}
