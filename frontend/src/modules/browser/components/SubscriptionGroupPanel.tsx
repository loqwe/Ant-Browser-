import { useEffect, useMemo, useState } from 'react'
import { Card, Select, toast } from '../../../shared/components'
import { fetchBrowserProxies, fetchBrowserSubscriptions, saveBrowserSubscription } from '../api'
import { parseSelectedProxyGroupMap, parseSubscriptionProxyGroups, resolveCurrentUpstream, resolveRecommendedUpstream } from '../subscriptionSelectors'
import { resolveActionErrorMessage } from '../utils/actionErrors'
import type { BrowserProxy, BrowserSubscriptionSource } from '../types'

interface SubscriptionGroupPanelProps {
  sourceId?: string
  onChanged?: () => void | Promise<void>
}

export function SubscriptionGroupPanel({ sourceId, onChanged }: SubscriptionGroupPanelProps) {
  const [sources, setSources] = useState<BrowserSubscriptionSource[]>([])
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [savingKey, setSavingKey] = useState('')

  const loadData = async () => {
    const [sourceList, proxyList] = await Promise.all([fetchBrowserSubscriptions(), fetchBrowserProxies()])
    setSources(sourceList)
    setProxies(proxyList.filter(proxy => !!proxy.sourceId))
  }
  useEffect(() => { void loadData() }, [])

  const handleUpdate = async (source: BrowserSubscriptionSource, groupName: string, proxyName: string) => {
    const key = `${source.sourceId}::${groupName}`
    setSavingKey(key)
    try {
      const selected = parseSelectedProxyGroupMap(source.selectedProxyGroupsJson)
      if (proxyName.trim()) selected[groupName] = proxyName.trim()
      else delete selected[groupName]
      await saveBrowserSubscription({ ...source, selectedProxyGroupsJson: JSON.stringify(selected) })
      toast.success('代理组已更新')
      await loadData()
      await onChanged?.()
    } catch (error) {
      toast.error(resolveActionErrorMessage(error, '更新代理组失败'))
    } finally {
      setSavingKey('')
    }
  }

  const items = useMemo(() => sources
    .filter(source => !sourceId || source.sourceId === sourceId)
    .map(source => {
    const sourceProxies = proxies.filter(proxy => proxy.sourceId === source.sourceId)
    const selectedMap = parseSelectedProxyGroupMap(source.selectedProxyGroupsJson)
    const selectGroups = parseSubscriptionProxyGroups(source.proxyGroupsJson).filter(group => (group.type || '').toLowerCase() === 'select' && Array.isArray(group.proxies) && group.proxies.length > 0)
    return { source, sourceProxies, selectedMap, selectGroups }
  }).filter(item => item.selectGroups.length > 0), [proxies, sourceId, sources])

  if (items.length === 0) {
    return <div className="rounded-2xl border border-dashed border-[var(--color-border)] bg-[var(--color-bg-surface)] px-6 py-12 text-center text-sm text-[var(--color-text-muted)]">暂无可配置的代理组，请先刷新订阅。</div>
  }

  return <div className="space-y-4">{items.map(({ source, sourceProxies, selectedMap, selectGroups }) => <Card key={source.sourceId} title={source.name || '未命名订阅'} subtitle={source.url}><div className="grid gap-4 md:grid-cols-2">{selectGroups.map(group => { const currentUpstream = resolveCurrentUpstream(group.name, sourceProxies, selectedMap); const recommendedUpstream = resolveRecommendedUpstream(group, sourceProxies); const key = `${source.sourceId}::${group.name}`; return <div key={key} className="space-y-1.5"><div className="text-sm font-medium text-[var(--color-text-primary)]">{group.name}</div><Select value={selectedMap[group.name] || currentUpstream || ''} onChange={event => void handleUpdate(source, group.name, event.target.value)} disabled={savingKey === key} options={[{ value: '', label: '自动选择（推荐）' }, ...(group.proxies || []).map(proxyName => ({ value: proxyName, label: proxyName }))]} /><div className="flex flex-wrap gap-2 text-xs text-[var(--color-text-muted)]"><span>推荐：{recommendedUpstream || '?'}</span><span>当前：{currentUpstream || '未解析'}</span></div></div> })}</div></Card>)}</div>
}
