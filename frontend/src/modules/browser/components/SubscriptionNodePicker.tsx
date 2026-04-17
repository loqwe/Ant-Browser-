import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ExternalLink, Search } from 'lucide-react'
import { Badge, Button, Input, Select } from '../../../shared/components'
import { fetchBrowserProxies, fetchBrowserSubscriptions } from '../api'
import type { BrowserProxy, BrowserSubscriptionSource } from '../types'

interface SubscriptionNodePickerProps {
  currentProxyId: string
  onSelect: (proxy: BrowserProxy) => void
}

const ALL = '__all__'
const resolveGroup = (proxy: BrowserProxy) => proxy.displayGroup || proxy.rawProxyGroupName || proxy.groupName || '未分组'

export function SubscriptionNodePicker({ currentProxyId, onSelect }: SubscriptionNodePickerProps) {
  const navigate = useNavigate()
  const [sources, setSources] = useState<BrowserSubscriptionSource[]>([])
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [selectedSource, setSelectedSource] = useState(ALL)
  const [selectedGroup, setSelectedGroup] = useState(ALL)
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    let alive = true
    const loadData = async () => {
      setLoading(true)
      try {
        const [sourceList, proxyList] = await Promise.all([fetchBrowserSubscriptions(), fetchBrowserProxies()])
        if (!alive) return
        setSources(sourceList)
        setProxies(proxyList.filter(proxy => !!proxy.sourceId))
        const current = proxyList.find(proxy => proxy.proxyId === currentProxyId && proxy.sourceId)
        setSelectedSource(current?.sourceId || ALL)
        setSelectedGroup(current ? resolveGroup(current) : ALL)
      } finally {
        if (alive) setLoading(false)
      }
    }
    loadData()
    return () => { alive = false }
  }, [currentProxyId])

  const sourceNameMap = useMemo(() => Object.fromEntries(sources.map(source => [source.sourceId, source.name])), [sources])
  const proxyNameMap = useMemo(() => Object.fromEntries(proxies.map(proxy => [proxy.proxyId, proxy.sourceNodeName || proxy.proxyName || proxy.proxyId])), [proxies])
  const visibleBySource = useMemo(() => selectedSource === ALL ? proxies : proxies.filter(proxy => proxy.sourceId === selectedSource), [proxies, selectedSource])
  const groupCounts = useMemo(() => visibleBySource.reduce<Record<string, number>>((acc, proxy) => {
    const group = resolveGroup(proxy)
    acc[group] = (acc[group] || 0) + 1
    return acc
  }, {}), [visibleBySource])
  const groupOptions = useMemo(() => [ALL, ...Object.keys(groupCounts).sort((a, b) => a.localeCompare(b, 'zh-CN'))], [groupCounts])
  const filtered = useMemo(() => {
    const keyword = search.trim().toLowerCase()
    return visibleBySource.filter(proxy => {
      if (selectedGroup !== ALL && resolveGroup(proxy) !== selectedGroup) return false
      if (!keyword) return true
      return [proxy.proxyName, proxy.sourceNodeName, proxy.upstreamAlias, proxy.rawProxyGroupName, proxyNameMap[proxy.upstreamProxyId || ''], sourceNameMap[proxy.sourceId || '']]
        .filter(Boolean)
        .some(value => String(value).toLowerCase().includes(keyword))
    }).sort((a, b) => (a.proxyName || a.proxyId).localeCompare(b.proxyName || b.proxyId, 'zh-CN'))
  }, [proxyNameMap, search, selectedGroup, sourceNameMap, visibleBySource])

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex flex-wrap items-center gap-2 border-b border-[var(--color-border)] px-5 py-3">
        <Select value={selectedSource} onChange={e => { setSelectedSource(e.target.value); setSelectedGroup(ALL) }} className="w-56" options={[{ value: ALL, label: '全部订阅源' }, ...sources.map(source => ({ value: source.sourceId, label: source.name }))]} />
        <div className="relative min-w-[260px] flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-muted)]" />
          <Input value={search} onChange={e => setSearch(e.target.value)} placeholder="搜索节点名 / 来源 / 上游" className="pl-9" />
        </div>
        <Button variant="secondary" size="sm" onClick={() => navigate('/browser/subscriptions')}><ExternalLink className="w-3.5 h-3.5" />管理订阅</Button>
      </div>
      <div className="grid min-h-0 flex-1 grid-cols-[220px_minmax(0,1fr)]">
        <div className="overflow-y-auto border-r border-[var(--color-border)] p-2">
          {groupOptions.map(group => (
            <button key={group} onClick={() => setSelectedGroup(group)} className={`flex w-full items-center justify-between rounded-lg px-3 py-2 text-left text-sm ${selectedGroup === group ? 'bg-[var(--color-primary)]/10 text-[var(--color-primary)]' : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-hover)]'}`}>
              <span className="truncate">{group === ALL ? '全部分组' : group}</span>
              <span className="text-xs opacity-70">{group === ALL ? visibleBySource.length : groupCounts[group] || 0}</span>
            </button>
          ))}
        </div>
        <div className="overflow-y-auto">
          {loading ? <div className="flex h-32 items-center justify-center text-sm text-[var(--color-text-muted)]">加载订阅节点...</div> : filtered.length === 0 ? <div className="flex h-32 items-center justify-center text-sm text-[var(--color-text-muted)]">没有匹配节点</div> : filtered.map(proxy => (
            <button key={proxy.proxyId} onClick={() => onSelect(proxy)} className={`flex w-full items-center justify-between gap-3 border-b border-[var(--color-border)]/40 px-4 py-3 text-left ${proxy.proxyId === currentProxyId ? 'bg-[var(--color-primary)]/10' : 'hover:bg-[var(--color-bg-hover)]'}`}>
              <div className="min-w-0 flex-1">
                <div className="truncate text-sm font-medium text-[var(--color-text-primary)]">{proxy.proxyName || proxy.proxyId}</div>
                <div className="mt-1 flex flex-wrap gap-2 text-xs text-[var(--color-text-muted)]">
                  <Badge size="sm">{sourceNameMap[proxy.sourceId || ''] || '未知订阅'}</Badge>
                  <Badge size="sm">{resolveGroup(proxy)}</Badge>
                  {proxy.chainMode === 'chained' && <Badge size="sm" variant="info">链式</Badge>}
                  {proxy.upstreamAlias && <span className="truncate">上游：{proxy.upstreamAlias}</span>}
                  {proxy.upstreamProxyId && <span className="truncate">真实上游：{proxyNameMap[proxy.upstreamProxyId] || '未解析'}</span>}
                </div>
              </div>
              {proxy.proxyId === currentProxyId && <Badge variant="success">当前绑定</Badge>}
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
