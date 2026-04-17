import { useMemo } from 'react'
import clsx from 'clsx'
import { Button, Card } from '../../../shared/components'
import { SubscriptionGroupPanel } from './SubscriptionGroupPanel'
import { SubscriptionRefreshStatus } from './SubscriptionRefreshStatus'
import type { BrowserProxy, BrowserSubscriptionSource, SubscriptionRefreshState } from '../types'

interface SubscriptionCatalogPanelProps {
  sources: BrowserSubscriptionSource[]
  proxies: BrowserProxy[]
  selectedSourceId: string
  importUrl: string
  quickImporting: boolean
  refreshingSourceIds: Set<string>
  refreshStates: Record<string, SubscriptionRefreshState | undefined>
  onImportUrlChange: (value: string) => void
  onQuickImport: () => void | Promise<void>
  onNew: () => void
  onSelectSource: (sourceId: string) => void
  onRefresh: (sourceId: string) => void | Promise<void>
  onEdit: (source: BrowserSubscriptionSource) => void
  onDelete: (source: BrowserSubscriptionSource) => void
  onOpenImport: (source: BrowserSubscriptionSource) => void
  onChanged?: () => void | Promise<void>
}

function getTimeAgo(dateString: string) {
  if (!dateString) return '未刷新'
  const time = Date.parse(dateString)
  if (!Number.isFinite(time)) return '未刷新'
  const diffMs = Date.now() - time
  if (diffMs < 60_000) return '刚刚'
  const diffMins = Math.floor(diffMs / 60_000)
  if (diffMins < 60) return `${diffMins} 分钟前`
  const diffHours = Math.floor(diffMins / 60)
  if (diffHours < 24) return `${diffHours} 小时前`
  return `${Math.floor(diffHours / 24)} 天前`
}


function parseImportStats(raw?: string) {
  try {
    return raw?.trim() ? JSON.parse(raw) as { catalogTotal?: number; importedCount?: number; missingSelectedCount?: number } : {}
  } catch {
    return {}
  }
}

function sourceHostLabel(sourceURL: string) {
  try { return new URL(sourceURL).host || sourceURL } catch { return sourceURL || '-' }
}

export function SubscriptionCatalogPanel({ sources, proxies, selectedSourceId, importUrl, quickImporting, refreshingSourceIds, refreshStates, onImportUrlChange, onQuickImport, onNew, onSelectSource, onRefresh, onEdit, onDelete, onOpenImport, onChanged }: SubscriptionCatalogPanelProps) {
  const statsMap = useMemo(() => proxies.reduce<Record<string, number>>((acc, proxy) => {
    const sourceId = proxy.sourceId || ''
    if (!sourceId) return acc
    acc[sourceId] = (acc[sourceId] || 0) + 1
    return acc
  }, {}), [proxies])
  const selectedSource = useMemo(() => sources.find(item => item.sourceId === selectedSourceId) || sources[0] || null, [selectedSourceId, sources])

  return (
    <div className="space-y-5">
      <div className="flex items-center gap-3 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-2 px-3">
        <div className="flex flex-1 items-center rounded-md border border-transparent bg-[var(--color-bg-secondary)] px-3 py-2 focus-within:border-[var(--color-primary)]">
          <input type="text" value={importUrl} onChange={e => onImportUrlChange(e.target.value)} placeholder="订阅文件链接" className="flex-1 bg-transparent text-sm text-[var(--color-text-primary)] outline-none" />
          <button type="button" className="ml-2 text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]" title="粘贴" onClick={async () => { try { const text = await navigator.clipboard.readText(); if (text) onImportUrlChange(text) } catch {} }}>粘贴</button>
        </div>
        <Button variant="secondary" className="h-9 px-6 font-normal" onClick={() => void onQuickImport()} loading={quickImporting}>导入</Button>
        <Button className="h-9 px-6 font-normal" onClick={onNew}>新建</Button>
      </div>

      {sources.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-6 py-16 text-center text-sm text-[var(--color-text-muted)]">暂无订阅，请粘贴链接导入或新建。</div>
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {sources.map(source => {
            const sourceId = source.sourceId || ''
            const selected = !!selectedSource && selectedSource.sourceId === sourceId
            const domain = sourceHostLabel(source.url)
            const importStats = parseImportStats(source.importStatsJson)
            const catalogTotal = importStats.catalogTotal || 0
            const importedCount = importStats.importedCount || (statsMap[sourceId] || 0)
            const refreshState = refreshStates[sourceId]
            return (
              <Card key={sourceId} padding="sm" className={clsx('border transition-all', selected ? 'border-[var(--color-accent)] shadow-[var(--shadow-md)]' : 'border-[var(--color-border-default)]')}>
                <div
                  role="button"
                  tabIndex={0}
                  className="w-full cursor-pointer text-left"
                  onClick={() => onSelectSource(sourceId)}
                  onKeyDown={event => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault()
                      onSelectSource(sourceId)
                    }
                  }}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="truncate text-base font-semibold text-[var(--color-text-primary)]">{source.name || '未命名订阅'}</div>
                      <div className="mt-1 truncate text-sm text-[var(--color-text-muted)]">{domain}</div>
                    </div>
                    <button
                      type="button"
                      className={clsx('shrink-0 text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]', refreshingSourceIds.has(sourceId) && 'animate-spin')}
                      title="刷新订阅"
                      onClick={e => {
                        e.stopPropagation()
                        void onRefresh(sourceId)
                      }}
                    >
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/></svg>
                    </button>
                  </div>
                  <div className="mt-4 flex items-center justify-between text-sm text-[var(--color-text-muted)]"><span>{catalogTotal > 0 ? `${catalogTotal} 个目录节点` : `${statsMap[sourceId] || 0} 个节点`}</span><span>{getTimeAgo(source.lastRefreshAt)}</span></div>
                  <div className="mt-2 flex items-center justify-between text-xs text-[var(--color-text-muted)]"><span>{source.enabled ? "已启用" : "已停用"} · 已导入 {importedCount}</span><span>{source.lastError ? "最近失败" : "可用"}</span></div>
                  <SubscriptionRefreshStatus state={refreshState} />
                  <div className="mt-3 h-1 rounded-full bg-[var(--color-bg-secondary)]">{(importedCount || statsMap[sourceId] || 0) > 0 && <div className="h-full w-full rounded-full bg-[var(--color-accent)]" />}</div>
                </div>
              </Card>
            )
          })}
        </div>
      )}

      <Card title="代理组" subtitle={selectedSource ? `当前订阅：${selectedSource.name || sourceHostLabel(selectedSource.url)}` : '请先选择一个订阅'} actions={selectedSource ? <><Button size="sm" variant="secondary" onClick={() => onOpenImport(selectedSource)}>选择导入</Button><Button size="sm" variant="secondary" onClick={() => onEdit(selectedSource)}>编辑</Button><Button size="sm" variant="danger" onClick={() => onDelete(selectedSource)}>删除</Button></> : undefined}>
        {selectedSource ? <SubscriptionGroupPanel sourceId={selectedSource.sourceId} onChanged={onChanged} /> : <div className="rounded-xl border border-dashed border-[var(--color-border-default)] px-6 py-12 text-center text-sm text-[var(--color-text-muted)]">点击上方订阅卡片后，这里会切换到对应订阅的代理组。</div>}
      </Card>
    </div>
  )
}
