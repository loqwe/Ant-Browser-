import { useMemo } from 'react'
import { Edit3, RefreshCw, Trash2 } from 'lucide-react'
import { Badge, Button, Card } from '../../../shared/components'
import type { BrowserProxy, BrowserSubscriptionSource } from '../types'

interface SubscriptionSourceCardProps {
  source: BrowserSubscriptionSource
  nodeCount: number
  groupCount: number
  refreshing: boolean
  sourceProxies: BrowserProxy[]
  selectionSavingKey?: string
  onEdit: () => void
  onRefresh: () => void
  onDelete: () => void
  onOpenImport: () => void
  onUpdateSelection?: (groupName: string, proxyName: string) => void | Promise<void>
}

function resolveStatusVariant(status: string) {
  if (status === 'ok') return 'success' as const
  if (status) return 'error' as const
  return 'default' as const
}

function resolveStatusLabel(source: BrowserSubscriptionSource) {
  if (source.lastRefreshStatus === 'ok') return '已刷新'
  if (source.lastRefreshStatus) return source.lastRefreshStatus
  return '未刷新'
}

function parseImportStats(raw?: string) {
  try { return raw?.trim() ? JSON.parse(raw) as { catalogTotal?: number; importedCount?: number; missingSelectedCount?: number } : {} } catch { return {} }
}

export function SubscriptionSourceCard({ source, nodeCount, groupCount, refreshing, onEdit, onRefresh, onDelete, onOpenImport }: SubscriptionSourceCardProps) {
  const importStats = useMemo(() => parseImportStats(source.importStatsJson), [source.importStatsJson])

  return (
    <Card title={source.name || '未命名订阅'} subtitle={source.url} actions={<><Button variant="secondary" size="sm" onClick={onOpenImport}>选择导入</Button><Button variant="secondary" size="sm" onClick={onEdit}><Edit3 className="w-3.5 h-3.5" />编辑</Button><Button variant="secondary" size="sm" onClick={onRefresh} loading={refreshing}><RefreshCw className="w-3.5 h-3.5" />刷新</Button><Button variant="danger" size="sm" onClick={onDelete}><Trash2 className="w-3.5 h-3.5" />删除</Button></>}>
      <div className="space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant={source.enabled ? 'success' : 'default'}>{source.enabled ? '启用' : '停用'}</Badge>
          <Badge variant={resolveStatusVariant(source.lastRefreshStatus)}>{resolveStatusLabel(source)}</Badge>
          <Badge>{importStats.catalogTotal || nodeCount} 目录节点</Badge>
          <Badge variant="success">{nodeCount} 已导入</Badge>
          <Badge>{groupCount} 分组</Badge>
          <Badge>{Math.max(1, source.refreshIntervalMinutes || 60)} 分钟刷新</Badge>
          {importStats.missingSelectedCount ? <Badge variant="warning">{importStats.missingSelectedCount} 已失效</Badge> : null}
        </div>
        <div className="grid gap-2 text-sm text-[var(--color-text-secondary)] md:grid-cols-3">
          <div>最近刷新：{source.lastRefreshAt || '未刷新'}</div>
          <div>流量使用：{source.trafficUsed || '--'} / {source.trafficTotal || '--'}</div>
          <div>到期时间：{source.expireAt || '--'}</div>
        </div>
        <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-surface)]/60 px-4 py-3 text-xs text-[var(--color-text-muted)]">代理组设置已拆分到“代理组”面板。</div>
        {source.lastError && <div className="rounded-lg border border-[var(--color-error)]/20 bg-[var(--color-error)]/5 px-3 py-2 text-sm text-[var(--color-error)]">{source.lastError}</div>}
      </div>
    </Card>
  )
}
