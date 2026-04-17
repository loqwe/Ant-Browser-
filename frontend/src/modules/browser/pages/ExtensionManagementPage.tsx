import { useEffect, useMemo, useState } from 'react'
import { ExternalLink, Globe2, MoreVertical, Plus, Puzzle, RefreshCw, Settings2, Trash2 } from 'lucide-react'
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime'
import { Button, Card, Input, toast } from '../../../shared/components'
import { deleteBrowserExtension, fetchBrowserExtensions, fetchBrowserProfileExtensions, fetchBrowserProfiles, importBrowserExtensionDir, importBrowserExtensionFromChromeStore, importBrowserExtensionPackage, pickBrowserExtensionDir, pickBrowserExtensionPackage, refreshBrowserExtension, setBrowserExtensionDefaultScope } from '../api'
import { ExtensionAssignmentModal } from '../components/ExtensionAssignmentModal'
import { ExtensionUploadModal } from '../components/ExtensionUploadModal'
import type { BrowserExtension } from '../types'

type ViewMode = 'installed' | 'center'
type SourceMode = 'all' | 'chrome' | 'firefox'
type CenterItem = { name: string; developer: string; source: Exclude<SourceMode, 'all'>; category: string; description: string; storeUrl: string; extensionId?: string }

const CENTER_ITEMS: CenterItem[] = [
  { name: 'Cookie-Editor', developer: 'cgagnier', source: 'chrome', category: '效率', description: '快速查看、编辑与导出 Cookie。', storeUrl: 'https://chromewebstore.google.com/detail/cookie-editor/hlkenndednhfkekhgcdicdfddnkalmdm', extensionId: 'hlkenndednhfkekhgcdicdfddnkalmdm' },
  { name: 'Google 翻译', developer: 'Google', source: 'chrome', category: '效率', description: '在网页中快速翻译选中文本和整页内容。', storeUrl: 'https://chromewebstore.google.com/detail/google-translate/aapbdbdomjkkjkaonfhkkikfgjllcleb', extensionId: 'aapbdbdomjkkjkaonfhkkikfgjllcleb' },
  { name: 'WebChatGPT', developer: 'webchatgpt.app', source: 'chrome', category: 'AI', description: '为 ChatGPT 提供联网搜索结果参考。', storeUrl: 'https://chromewebstore.google.com/detail/webchatgpt/lpfemeioodjbpieminkklglpmhlngfcn', extensionId: 'lpfemeioodjbpieminkklglpmhlngfcn' },
  { name: 'Facebook Pixel Helper', developer: 'Meta', source: 'chrome', category: '营销', description: '辅助检测 Pixel 安装与事件触发状态。', storeUrl: 'https://chromewebstore.google.com/detail/facebook-pixel-helper/fdgfkebogiimcoedlicjlajpkdmockpc', extensionId: 'fdgfkebogiimcoedlicjlajpkdmockpc' },
  { name: 'uBlock Origin', developer: 'Raymond Hill', source: 'firefox', category: '隐私', description: '内容拦截与隐私保护常用扩展。', storeUrl: 'https://addons.mozilla.org/zh-CN/firefox/addon/ublock-origin/' },
]

function cleanDisplayText(value: string | undefined, fallback: string) {
  const raw = (value || '').trim()
  if (!raw) return fallback
  if (/__MSG_/i.test(raw) || /^_MSG_/i.test(raw)) return fallback
  return raw.replace(/^_+|_+$/g, '') || fallback
}

function detectExtensionId(item: BrowserExtension) {
  const sourcePath = (item.sourcePath || '').trim()
  const encoded = sourcePath.match(/id%3D([a-z]{32})%26/i)
  if (encoded?.[1]) return encoded[1]
  const detail = sourcePath.match(/\/detail\/[^/]+\/([a-z]{32})/i)
  if (detail?.[1]) return detail[1]
  return ''
}

function resolveInstalledDeveloper(item: BrowserExtension) {
  const extensionId = detectExtensionId(item)
  if (extensionId) {
    const preset = CENTER_ITEMS.find(entry => entry.extensionId === extensionId)
    if (preset?.developer) return preset.developer
  }
  const normalizedName = cleanDisplayText(item.name, '').toLowerCase()
  const presetByName = CENTER_ITEMS.find(entry => entry.name.toLowerCase() === normalizedName)
  if (presetByName?.developer) return presetByName.developer
  const sourcePath = (item.sourcePath || '').trim()
  if (/^https?:\/\//i.test(sourcePath)) {
    try { return new URL(sourcePath).origin.replace(/^https?:\/\//i, '') } catch { return sourcePath }
  }
  return '本地导入'
}

function resolveAssignmentLabel(item: BrowserExtension, count: number) {
  return item.enabledByDefault ? '全局生效' : count > 0 ? `手动分配 (${count})` : '手动分配'
}

function resolveSourceBadge(item: BrowserExtension) {
  return item.sourceType === 'crx' || item.sourceType === 'zip' ? 'C' : 'L'
}

function resolvePrimaryText(item: BrowserExtension) {
  return cleanDisplayText(item.name, '未命名扩展')
}

function resolveSecondaryText(item: BrowserExtension) {
  return cleanDisplayText(item.description, '暂无描述')
}

export function ExtensionManagementPage() {
  const [viewMode, setViewMode] = useState<ViewMode>('installed')
  const [items, setItems] = useState<BrowserExtension[]>([])
  const [keyword, setKeyword] = useState('')
  const [sourceMode, setSourceMode] = useState<SourceMode>('all')
  const [category, setCategory] = useState('全部')
  const [actionMenuId, setActionMenuId] = useState<string | null>(null)
  const [uploadModalOpen, setUploadModalOpen] = useState(false)
  const [assignTarget, setAssignTarget] = useState<BrowserExtension | null>(null)
  const [uploading, setUploading] = useState(false)
  const [assignmentStats, setAssignmentStats] = useState<Record<string, number>>({})

  const loadData = async () => {
    const extensions = await fetchBrowserExtensions()
    setItems(extensions)
    const profiles = await fetchBrowserProfiles()
    const entries = await Promise.all(profiles.map(async profile => [profile.profileId, await fetchBrowserProfileExtensions(profile.profileId)] as const))
    const stats: Record<string, number> = {}
    entries.forEach(([, bindings]) => bindings.forEach(binding => { if (binding.enabled) stats[binding.extensionId] = (stats[binding.extensionId] || 0) + 1 }))
    setAssignmentStats(stats)
  }

  useEffect(() => { void loadData() }, [])
  useEffect(() => {
    const close = () => setActionMenuId(null)
    window.addEventListener('click', close)
    return () => window.removeEventListener('click', close)
  }, [])

  const categories = useMemo(() => ['全部', ...Array.from(new Set(CENTER_ITEMS.map(item => item.category)))], [])
  const installed = useMemo(() => {
    const q = keyword.trim().toLowerCase()
    if (!q) return items
    return items.filter(item => [resolvePrimaryText(item), resolveSecondaryText(item), item.version].join(' ').toLowerCase().includes(q))
  }, [items, keyword])
  const centerItems = useMemo(() => CENTER_ITEMS.filter(item => (sourceMode === 'all' || item.source === sourceMode) && (category === '全部' || item.category === category) && (!keyword.trim() || [item.name, item.developer, item.description].join(' ').toLowerCase().includes(keyword.trim().toLowerCase()))), [sourceMode, category, keyword])

  const handleImported = async (item: BrowserExtension | null, actionText: string) => {
    if (!item) return
    toast.success(item.enabledByDefault ? `${actionText}：${resolvePrimaryText(item)}。已对全部实例默认生效，重启实例后加载` : `${actionText}：${resolvePrimaryText(item)}。请点击配置分配到实例、分组或标签`)
    await loadData()
    setViewMode('installed')
    setUploadModalOpen(false)
  }

  const handleSubmitStore = async (extensionId: string) => {
    setUploading(true)
    try { await handleImported(await importBrowserExtensionFromChromeStore(extensionId), '已加入扩展仓库') } catch (error: any) { toast.error(error?.message || '加入扩展仓库失败') } finally { setUploading(false) }
  }

  const handleSubmitCustom = async (mode: 'package' | 'dir', path: string) => {
    setUploading(true)
    try { await handleImported(mode === 'package' ? await importBrowserExtensionPackage(path) : await importBrowserExtensionDir(path), '已导入扩展') } catch (error: any) { toast.error(error?.message || '导入扩展失败') } finally { setUploading(false) }
  }

  const handleDelete = async (item: BrowserExtension) => {
    try { await deleteBrowserExtension(item.extensionId); toast.success(`已删除扩展：${resolvePrimaryText(item)}`); await loadData(); setActionMenuId(null) } catch (error: any) { toast.error(error?.message || '删除扩展失败') }
  }

  const handleRefresh = async (item: BrowserExtension) => {
    try { await refreshBrowserExtension(item.extensionId); toast.success(`已更新扩展：${resolvePrimaryText(item)}`); await loadData(); setActionMenuId(null) } catch (error: any) { toast.error(error?.message || '更新扩展失败') }
  }

  const handleSetGlobal = async (item: BrowserExtension, enabled: boolean) => {
    try { await setBrowserExtensionDefaultScope(item.extensionId, enabled); toast.success(enabled ? '已设为全局生效' : '已关闭全局生效'); await loadData(); setActionMenuId(null) } catch (error: any) { toast.error(error?.message || '更新分配方式失败') }
  }

  const handleCenterAction = async (item: CenterItem) => {
    if (item.source === 'chrome' && item.extensionId) {
      try { await handleImported(await importBrowserExtensionFromChromeStore(item.extensionId), '已加入扩展仓库') } catch (error: any) { toast.error(error?.message || '加入扩展仓库失败') }
      return
    }
    BrowserOpenURL(item.storeUrl)
  }

  return (
    <>
      <div className="space-y-4 animate-fade-in">
        <div className="rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-4 shadow-sm">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex flex-wrap items-center gap-2">
              <Button size="sm" onClick={() => setUploadModalOpen(true)}><Plus className="w-4 h-4" />上传扩展</Button>
              <Button size="sm" variant={viewMode === 'center' ? 'primary' : 'secondary'} onClick={() => setViewMode('center')}><Puzzle className="w-4 h-4" />扩展中心</Button>
              <Button size="sm" variant={viewMode === 'installed' ? 'primary' : 'secondary'} onClick={() => setViewMode('installed')}>已添加扩展</Button>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {viewMode === 'center' && <select value={category} onChange={e => setCategory(e.target.value)} className="h-9 rounded-lg border border-[var(--color-border-default)] bg-white px-3 text-sm">{categories.map(item => <option key={item} value={item}>{item}</option>)}</select>}
              {viewMode === 'center' && <div className="inline-flex overflow-hidden rounded-lg border border-[var(--color-border-default)]">{([{ key: 'all', label: '全部' }, { key: 'chrome', label: '谷歌' }, { key: 'firefox', label: '火狐' }] as const).map(item => <button key={item.key} onClick={() => setSourceMode(item.key)} className={`px-4 py-2 text-sm ${sourceMode === item.key ? 'bg-[var(--color-accent)] text-white' : 'bg-white text-[var(--color-text-secondary)]'}`}>{item.label}</button>)}</div>}
              <Input value={keyword} onChange={e => setKeyword(e.target.value)} placeholder={viewMode === 'center' ? '请输入扩展名称' : '搜索已添加扩展'} className="w-64" />
            </div>
          </div>
        </div>

        {viewMode === 'installed' ? (
          <Card title="已添加扩展" subtitle="支持全局生效、手动分配及按实例 / 分组 / 标签分配">
            {installed.length === 0 ? (
              <div className="flex min-h-[260px] flex-col items-center justify-center gap-4 text-center text-[var(--color-text-muted)]"><Puzzle className="h-10 w-10 text-[var(--color-accent)]" /><div className="text-2xl font-light text-[var(--color-text-primary)]">暂无数据</div><Button onClick={() => setUploadModalOpen(true)}><Plus className="w-4 h-4" />上传扩展</Button></div>
            ) : (
              <div className="overflow-visible rounded-2xl border border-[var(--color-border-default)] bg-white">
                <div className="grid grid-cols-[minmax(0,3.1fr)_1.4fr_1.1fr_160px] border-b border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-5 py-3 text-[14px] font-medium text-[var(--color-text-secondary)]"><div>扩展</div><div>开发者</div><div>分配方式</div><div className="text-right">操作</div></div>
                {installed.map((item, index) => {
                  const title = resolvePrimaryText(item)
                  const description = resolveSecondaryText(item)
                  const developer = resolveInstalledDeveloper(item)
                  const assignment = resolveAssignmentLabel(item, assignmentStats[item.extensionId] || 0)
                  const openUpward = index >= installed.length - 2
                  return (
                    <div key={item.extensionId} className="grid grid-cols-[minmax(0,3.1fr)_1.4fr_1.1fr_160px] items-center border-b border-[var(--color-border-muted)] px-5 py-3.5 last:border-b-0 hover:bg-[var(--color-bg-secondary)]/40 transition-colors">
                      <div className="flex min-w-0 items-center gap-3">
                        <div className="relative flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-[var(--color-bg-secondary)] text-[15px] font-semibold text-[var(--color-accent)] shadow-sm">{title.slice(0, 2)}<span className="absolute -bottom-1 -left-1 inline-flex h-4 min-w-4 items-center justify-center rounded-full border border-white bg-[#4285F4] px-1 text-[9px] font-bold text-white shadow">{resolveSourceBadge(item)}</span></div>
                        <div className="min-w-0">
                          <div className="truncate text-[14px] font-semibold text-[var(--color-text-primary)]">{title}</div>
                          <div className="mt-0.5 truncate text-[13px] text-[var(--color-text-muted)]">{description}</div>
                          <div className="mt-0.5 text-[12px] text-[var(--color-text-muted)]">{item.version || '未知版本'}</div>
                        </div>
                      </div>
                      <div className="min-w-0 pr-4 text-[14px] text-[var(--color-text-secondary)] break-all">{developer}</div>
                      <div><span className={`inline-flex rounded-full px-3 py-1 text-[13px] font-medium ${item.enabledByDefault ? 'bg-[var(--color-accent-muted)] text-[var(--color-accent)]' : 'bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)]'}`}>{assignment}</span></div>
                      <div className="relative flex items-center justify-end gap-2" onClick={(e) => e.stopPropagation()}>
                        {!item.enabledByDefault && <Button size="sm" variant="secondary" className="h-8 px-3 text-[12px]" onClick={() => setAssignTarget(item)}><Settings2 className="w-3.5 h-3.5" />配置</Button>}
                        <button type="button" className="inline-flex h-8 w-8 items-center justify-center rounded-full text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-secondary)] hover:text-[var(--color-text-primary)]" onClick={() => setActionMenuId(current => current === item.extensionId ? null : item.extensionId)}><MoreVertical className="h-4 w-4" /></button>
                        {actionMenuId === item.extensionId && (
                          <div className={`absolute right-0 ${openUpward ? 'bottom-9' : 'top-9'} z-20 min-w-[180px] overflow-hidden rounded-xl border border-[var(--color-border-default)] bg-white shadow-xl`}>
                            <button type="button" className="flex w-full items-center gap-2 px-4 py-3 text-left text-sm text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-secondary)]" onClick={() => void handleSetGlobal(item, !item.enabledByDefault)}><Globe2 className="h-4 w-4" />{item.enabledByDefault ? '关闭全局生效' : '设为全局生效'}</button>
                            <button type="button" className="flex w-full items-center gap-2 px-4 py-3 text-left text-sm text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-secondary)]" onClick={() => void handleRefresh(item)}><RefreshCw className="h-4 w-4" />更新</button>
                            <button type="button" className="flex w-full items-center gap-2 px-4 py-3 text-left text-sm text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-secondary)]" onClick={() => void handleDelete(item)}><Trash2 className="h-4 w-4" />删除</button>
                          </div>
                        )}
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </Card>
        ) : (
          <div className="space-y-5">
            <div className="text-center text-[var(--color-accent)] text-2xl font-semibold tracking-wide">—— 热门推荐 ——</div>
            <div className="grid grid-cols-1 gap-5 md:grid-cols-2 xl:grid-cols-3">
              {centerItems.map(item => (
                <div key={`${item.source}-${item.name}`} className="rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-5 shadow-sm transition-all duration-200 hover:-translate-y-1 hover:shadow-lg">
                  <div className="mb-4 flex items-start gap-4"><div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-[var(--color-bg-secondary)] text-xl font-semibold text-[var(--color-accent)]">{item.name.slice(0, 2)}</div><div className="min-w-0"><div className="truncate text-2xl font-semibold text-[var(--color-text-primary)]">{item.name}</div><div className="mt-1 text-sm text-[var(--color-text-secondary)]">开发者：{item.developer}</div><div className="mt-1 text-sm text-[var(--color-text-muted)]">支持内核：{item.source === 'chrome' ? 'Chrome' : 'Firefox'}</div></div></div>
                  <div className="mb-5 min-h-[72px] text-[15px] leading-7 text-[var(--color-text-secondary)]">{item.description}</div>
                  <Button size="sm" className="w-full" onClick={() => handleCenterAction(item)}>{item.source === 'chrome' && item.extensionId ? '一键加入扩展仓库' : <><ExternalLink className="w-4 h-4" />前往商店</>}</Button>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      <ExtensionUploadModal open={uploadModalOpen} submitting={uploading} onClose={() => { if (!uploading) setUploadModalOpen(false) }} onPickPackage={pickBrowserExtensionPackage} onPickDir={pickBrowserExtensionDir} onSubmitStore={handleSubmitStore} onSubmitCustom={handleSubmitCustom} />
      <ExtensionAssignmentModal open={!!assignTarget} extension={assignTarget} onClose={() => setAssignTarget(null)} onSaved={loadData} />
    </>
  )
}
