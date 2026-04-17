package backend

import (
	"fmt"
	"strings"

	"ant-chrome/backend/internal/logger"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) BrowserExtensionList() []BrowserExtension {
	return a.browserMgr.ListExtensions()
}

func (a *App) BrowserExtensionImportDir(dir string) (*BrowserExtension, error) {
	log := logger.New("Extension")
	if strings.TrimSpace(dir) == "" {
		selected, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "选择扩展目录"})
		if err != nil {
			log.Error("选择扩展目录失败", logger.F("error", err.Error()))
			return nil, fmt.Errorf("选择扩展目录失败: %w", err)
		}
		dir = selected
	}
	if strings.TrimSpace(dir) == "" {
		log.Warn("扩展目录导入取消")
		return nil, fmt.Errorf("未选择扩展目录")
	}
	dir = strings.TrimSpace(dir)
	log.Info("收到扩展目录导入请求", logger.F("dir", dir))
	item, err := a.browserMgr.ImportExtensionDir(dir)
	if err != nil {
		log.Error("扩展目录导入失败", logger.F("dir", dir), logger.F("error", err.Error()))
		return nil, err
	}
	log.Info("扩展目录导入成功", logger.F("extension_id", item.ExtensionId), logger.F("name", item.Name))
	return item, nil
}

func (a *App) BrowserExtensionImportCRX(path string) (*BrowserExtension, error) {
	log := logger.New("Extension")
	if strings.TrimSpace(path) == "" {
		selected, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "选择扩展文件", Filters: []wailsruntime.FileFilter{{DisplayName: "扩展文件", Pattern: "*.crx;*.zip"}}})
		if err != nil {
			log.Error("选择扩展文件失败", logger.F("error", err.Error()))
			return nil, fmt.Errorf("选择扩展文件失败: %w", err)
		}
		path = selected
	}
	if strings.TrimSpace(path) == "" {
		log.Warn("扩展文件导入取消")
		return nil, fmt.Errorf("未选择扩展文件")
	}
	path = strings.TrimSpace(path)
	log.Info("收到扩展文件导入请求", logger.F("path", path))
	item, err := a.browserMgr.ImportExtensionCRX(path)
	if err != nil {
		log.Error("扩展文件导入失败", logger.F("path", path), logger.F("error", err.Error()))
		return nil, err
	}
	log.Info("扩展文件导入成功", logger.F("extension_id", item.ExtensionId), logger.F("name", item.Name))
	return item, nil
}

func (a *App) BrowserExtensionImportURL(url string) (*BrowserExtension, error) {
	log := logger.New("Extension")
	if strings.TrimSpace(url) == "" {
		log.Warn("URL 导入请求缺少下载地址")
		return nil, fmt.Errorf("下载地址不能为空")
	}
	url = strings.TrimSpace(url)
	log.Info("收到扩展 URL 导入请求", logger.F("url", url))
	item, err := a.browserMgr.ImportExtensionURL(url)
	if err != nil {
		log.Error("扩展 URL 导入失败", logger.F("url", url), logger.F("error", err.Error()))
		return nil, err
	}
	log.Info("扩展 URL 导入成功", logger.F("extension_id", item.ExtensionId), logger.F("name", item.Name))
	return item, nil
}

func (a *App) BrowserExtensionImportChromeStore(extensionId string) (*BrowserExtension, error) {
	log := logger.New("Extension")
	if strings.TrimSpace(extensionId) == "" {
		log.Warn("商店扩展导入请求缺少 extensionId")
		return nil, fmt.Errorf("扩展 ID 不能为空")
	}
	extensionId = strings.TrimSpace(extensionId)
	log.Info("收到 Chrome 商店扩展导入请求", logger.F("extension_id", extensionId))
	item, err := a.browserMgr.ImportChromeWebStoreExtension(extensionId)
	if err != nil {
		log.Error("Chrome 商店扩展导入失败", logger.F("extension_id", extensionId), logger.F("error", err.Error()))
		return nil, err
	}
	log.Info("Chrome 商店扩展导入成功", logger.F("extension_id", item.ExtensionId), logger.F("name", item.Name))
	return item, nil
}

func (a *App) BrowserExtensionSelectDir() (string, error) {
	log := logger.New("Extension")
	selected, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "选择扩展目录"})
	if err != nil {
		log.Error("选择扩展目录失败", logger.F("error", err.Error()))
		return "", fmt.Errorf("选择扩展目录失败: %w", err)
	}
	return strings.TrimSpace(selected), nil
}

func (a *App) BrowserExtensionSelectCRX() (string, error) {
	log := logger.New("Extension")
	selected, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "选择扩展文件", Filters: []wailsruntime.FileFilter{{DisplayName: "扩展文件", Pattern: "*.crx;*.zip"}}})
	if err != nil {
		log.Error("选择扩展文件失败", logger.F("error", err.Error()))
		return "", fmt.Errorf("选择扩展文件失败: %w", err)
	}
	return strings.TrimSpace(selected), nil
}
func (a *App) BrowserExtensionDelete(extensionId string) error {
	return a.browserMgr.DeleteExtension(extensionId)
}

func (a *App) BrowserExtensionSetDefaultScope(extensionId string, enabled bool) (*BrowserExtension, error) {
	return a.browserMgr.SetExtensionEnabledByDefault(extensionId, enabled)
}

func (a *App) BrowserExtensionRefresh(extensionId string) (*BrowserExtension, error) {
	return a.browserMgr.RefreshExtension(extensionId)
}

func (a *App) BrowserProfileExtensionList(profileId string) []BrowserProfileExtensionBinding {
	return a.browserMgr.ListProfileExtensions(profileId)
}

func (a *App) BrowserProfileExtensionSave(profileId string, bindings []BrowserProfileExtensionBinding) error {
	return a.browserMgr.SaveProfileExtensions(profileId, bindings)
}
