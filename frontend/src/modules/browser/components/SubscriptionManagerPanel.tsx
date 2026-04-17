import { useEffect, useMemo, useState } from 'react'
import { Plus } from 'lucide-react'
import { Button, ConfirmModal, toast } from '../../../shared/components'
import { deleteBrowserSubscription, fetchBrowserProxies, fetchBrowserSubscriptions, refreshBrowserSubscription, saveBrowserSubscription } from '../api'
import { resolveActionErrorMessage, resolveActionErrorSummary } from '../utils/actionErrors'
import { SubscriptionImportPicker } from './SubscriptionImportPicker'
import { SubscriptionSourceCard } from './SubscriptionSourceCard'
import { SubscriptionSourceModal } from './SubscriptionSourceModal'
import type { BrowserProxy, BrowserSubscriptionSource } from '../types'

interface SubscriptionManagerPanelProps {
  onChanged?: () => void | Promise<void>
}

function resolveGroup(proxy: BrowserProxy) {
  return proxy.displayGroup || proxy.rawProxyGroupName || proxy.groupName || '未分组'
}

function updateSelectedProxyGroups(source: BrowserSubscriptionSource, groupName: string, proxyName: string) {
  let selected: Record<string, string> = {}
  if (source.selectedProxyGroupsJson?.trim()) {
    try {
      const parsed = JSON.parse(source.selectedProxyGroupsJson)
      if (parsed && typeof parsed === 'object') selected = parsed
    } catch {
      selected = {}
    }
  }
  if (proxyName.trim()) selected[groupName] = proxyName.trim()
  else delete selected[groupName]
  return { ...source, selectedProxyGroupsJson: JSON.stringify(selected) }
}

export function SubscriptionManagerPanel({ onChanged }: SubscriptionManagerPanelProps) {
  const [sources, setSources] = useState<BrowserSubscriptionSource[]>([])
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [editorSource, setEditorSource] = useState<BrowserSubscriptionSource | null>(null)
  const [importSource, setImportSource] = useState<BrowserSubscriptionSource | null>(null)
  const [editorOpen, setEditorOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<BrowserSubscriptionSource | null>(null)
  const [saving, setSaving] = useState(false)
  const [refreshingId, setRefreshingId] = useState('')
  const [selectionSavingKey, setSelectionSavingKey] = useState('')

  const loadData = async () => {
    const [sourceList, proxyList] = await Promise.all([fetchBrowserSubscriptions(), fetchBrowserProxies()])
    setSources(sourceList)
    setProxies(proxyList.filter(proxy => !!proxy.sourceId))
  }
  useEffect(() => { void loadData() }, [])

  const statsMap = useMemo(() => proxies.reduce<Record<string, { nodeCount: number; groupCount: number }>>((acc, proxy) => {
    const sourceId = proxy.sourceId || ''
    const current = acc[sourceId] || { nodeCount: 0, groupCount: 0 }
    const groups = new Set<string>((current as { groups?: string[] }).groups || [])
    groups.add(resolveGroup(proxy))
    acc[sourceId] = { nodeCount: current.nodeCount + 1, groupCount: groups.size }
    ;(acc[sourceId] as { groups?: string[] }).groups = Array.from(groups)
    return acc
  }, {}), [proxies])

  const notifyChanged = async () => { if (onChanged) await onChanged() }
  const handleSave = async (source: BrowserSubscriptionSource) => {
    setSaving(true)
    try {
      await saveBrowserSubscription(source)
      toast.success(source.sourceId ? '订阅已更新' : '订阅已创建')
      setEditorOpen(false)
      setEditorSource(null)
      await loadData()
      await notifyChanged()
    } catch (error) { toast.error(resolveActionErrorMessage(error, '保存订阅失败')) } finally { setSaving(false) }
  }
  const handleRefresh = async (sourceId: string) => {
    setRefreshingId(sourceId)
    try {
      await refreshBrowserSubscription(sourceId)
      const refreshedAt = new Date().toISOString()
      setSources(prev => prev.map(item => item.sourceId === sourceId ? { ...item, lastRefreshAt: refreshedAt, lastRefreshStatus: 'ok', lastError: '' } : item))
      toast.success('订阅刷新完成')
      await loadData()
      await notifyChanged()
    } catch (error) { await loadData(); await notifyChanged(); toast.error(resolveActionErrorSummary(error, '??????')) } finally { setRefreshingId('') }
  }
  const handleSelectionChange = async (source: BrowserSubscriptionSource, groupName: string, proxyName: string) => {
    const key = `${source.sourceId}::${groupName}`
    setSelectionSavingKey(key)
    try {
      await saveBrowserSubscription(updateSelectedProxyGroups(source, groupName, proxyName))
      toast.success('链式上游已更新')
      await loadData()
      await notifyChanged()
    } catch (error) { toast.error(resolveActionErrorMessage(error, '更新链式上游失败')) } finally { setSelectionSavingKey('') }
  }
  const handleDelete = async () => {
    if (!deleteTarget) return
    try {
      await deleteBrowserSubscription(deleteTarget.sourceId)
      toast.success('订阅已删除')
      setDeleteTarget(null)
      await loadData()
      await notifyChanged()
    } catch (error) { toast.error(resolveActionErrorMessage(error, '删除订阅失败')) }
  }

  return <div className="space-y-4"><div className="flex flex-wrap items-center justify-between gap-3"><div className="text-sm text-[var(--color-text-muted)]">维护订阅源后，代理池会自动按订阅来源组织节点。</div><Button onClick={() => { setEditorSource(null); setEditorOpen(true) }}><Plus className="h-4 w-4" />新增订阅</Button></div>{sources.length === 0 ? <div className="rounded-2xl border border-dashed border-[var(--color-border)] bg-[var(--color-bg-surface)] px-6 py-12 text-center text-sm text-[var(--color-text-muted)]">还没有订阅源，先新增一个订阅再刷新节点。</div> : sources.map(source => { const stats = statsMap[source.sourceId] || { nodeCount: 0, groupCount: 0 }; const sourceProxies = proxies.filter(proxy => proxy.sourceId === source.sourceId); return <SubscriptionSourceCard key={source.sourceId} source={source} nodeCount={stats.nodeCount} groupCount={stats.groupCount} refreshing={refreshingId === source.sourceId} sourceProxies={sourceProxies} selectionSavingKey={selectionSavingKey} onEdit={() => { setEditorSource(source); setEditorOpen(true) }} onRefresh={() => void handleRefresh(source.sourceId)} onDelete={() => setDeleteTarget(source)} onOpenImport={() => { setImportSource(source); setImportOpen(true) }} onUpdateSelection={(groupName, proxyName) => handleSelectionChange(source, groupName, proxyName)} /> })}<SubscriptionSourceModal open={editorOpen} source={editorSource} saving={saving} onClose={() => { setEditorOpen(false); setEditorSource(null) }} onSubmit={handleSave} /><SubscriptionImportPicker open={importOpen} source={importSource} onClose={() => { setImportOpen(false); setImportSource(null) }} onSaved={async () => { await loadData(); await notifyChanged() }} /><ConfirmModal open={!!deleteTarget} onClose={() => setDeleteTarget(null)} onConfirm={() => void handleDelete()} title="删除订阅源？" content={`删除后将清理该订阅的全部节点：${deleteTarget?.name || ''}`} confirmText="确认删除" cancelText="取消" danger /></div>
}
