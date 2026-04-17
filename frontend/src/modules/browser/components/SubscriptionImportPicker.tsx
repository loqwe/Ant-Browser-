import { useEffect, useMemo, useState } from 'react'
import { Badge, Button, Input, Modal, Select, toast } from '../../../shared/components'
import { fetchBrowserSubscriptionNodes, saveBrowserSubscriptionSelectedNodes } from '../api'
import { resolveActionErrorMessage } from '../utils/actionErrors'
import type { BrowserSubscriptionNode, BrowserSubscriptionSource } from '../types'

interface SubscriptionImportPickerProps {
  open: boolean
  source: BrowserSubscriptionSource | null
  onClose: () => void
  onSaved?: () => void | Promise<void>
}

const ALL = '__all__'

function parseSelectedNodeKeys(raw?: string) {
  if (!raw?.trim()) return [] as string[]
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.map(item => String(item || '').trim()).filter(Boolean) : []
  } catch {
    return []
  }
}

export function SubscriptionImportPicker({ open, source, onClose, onSaved }: SubscriptionImportPickerProps) {
  const [nodes, setNodes] = useState<BrowserSubscriptionNode[]>([])
  const [selected, setSelected] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [search, setSearch] = useState('')
  const [group, setGroup] = useState(ALL)
  const [protocol, setProtocol] = useState(ALL)
  const [onlySelected, setOnlySelected] = useState(false)

  useEffect(() => {
    if (!open || !source?.sourceId) return
    setSelected(parseSelectedNodeKeys(source.selectedNodeKeysJson))
    setSearch('')
    setGroup(ALL)
    setProtocol(ALL)
    setOnlySelected(false)
    setLoading(true)
    void fetchBrowserSubscriptionNodes(source.sourceId).then(list => setNodes(list)).catch(error => toast.error(resolveActionErrorMessage(error, '加载订阅节点失败'))).finally(() => setLoading(false))
  }, [open, source?.sourceId, source?.selectedNodeKeysJson])

  const selectedSet = useMemo(() => new Set(selected), [selected])
  const groups = useMemo(() => [ALL, ...Array.from(new Set(nodes.map(node => node.displayGroup || '未分组'))).sort((a, b) => a.localeCompare(b, 'zh-CN'))], [nodes])
  const protocols = useMemo(() => [ALL, ...Array.from(new Set(nodes.map(node => node.protocol || 'unknown'))).sort()], [nodes])
  const filtered = useMemo(() => nodes.filter(node => {
    if (group !== ALL && (node.displayGroup || '未分组') !== group) return false
    if (protocol !== ALL && (node.protocol || 'unknown') !== protocol) return false
    if (onlySelected && !selectedSet.has(node.nodeKey)) return false
    const keyword = search.trim().toLowerCase()
    if (!keyword) return true
    return [node.nodeName, node.server, node.upstreamAlias, node.displayGroup, node.protocol].some(value => String(value || '').toLowerCase().includes(keyword))
  }), [group, nodes, onlySelected, protocol, search, selectedSet])

  const allVisibleSelected = filtered.length > 0 && filtered.every(node => selectedSet.has(node.nodeKey))
  const toggleNode = (nodeKey: string, checked: boolean) => setSelected(prev => checked ? Array.from(new Set([...prev, nodeKey])) : prev.filter(item => item !== nodeKey))
  const toggleVisible = (checked: boolean) => setSelected(prev => checked ? Array.from(new Set([...prev, ...filtered.map(node => node.nodeKey)])) : prev.filter(item => !filtered.some(node => node.nodeKey === item)))

  const handleSave = async () => {
    if (!source) return
    setSaving(true)
    try {
      await saveBrowserSubscriptionSelectedNodes(source, selected, true)
      toast.success('导入范围已更新')
      await onSaved?.()
      onClose()
    } catch (error) {
      toast.error(resolveActionErrorMessage(error, '保存导入范围失败'))
    } finally {
      setSaving(false)
    }
  }

  return <Modal open={open} onClose={onClose} title="选择导入节点" width="960px" footer={<><Button variant="secondary" onClick={onClose}>取消</Button><Button onClick={() => void handleSave()} loading={saving}>保存并同步代理池</Button></>}><div className="space-y-4"><div className="flex flex-wrap items-center gap-2"><Input value={search} onChange={e => setSearch(e.target.value)} placeholder="搜索节点名 / 服务器 / 上游" className="w-64" /><Select value={group} onChange={e => setGroup(e.target.value)} className="w-44" options={groups.map(item => ({ value: item, label: item === ALL ? '全部分组' : item }))} /><Select value={protocol} onChange={e => setProtocol(e.target.value)} className="w-36" options={protocols.map(item => ({ value: item, label: item === ALL ? '全部协议' : item.toUpperCase() }))} /><label className="ml-auto flex items-center gap-2 text-sm text-[var(--color-text-secondary)]"><input type="checkbox" checked={onlySelected} onChange={e => setOnlySelected(e.target.checked)} />仅看已选</label></div><div className="flex flex-wrap items-center gap-2 text-sm text-[var(--color-text-secondary)]"><Badge>{nodes.length} 目录节点</Badge><Badge variant="success">{selected.length} 已选</Badge><label className="flex items-center gap-2"><input type="checkbox" checked={allVisibleSelected} onChange={e => toggleVisible(e.target.checked)} disabled={filtered.length === 0} />全选当前筛选</label></div><div className="max-h-[55vh] overflow-y-auto rounded-xl border border-[var(--color-border)]">{loading ? <div className="p-6 text-center text-sm text-[var(--color-text-muted)]">加载中...</div> : filtered.length === 0 ? <div className="p-6 text-center text-sm text-[var(--color-text-muted)]">没有匹配节点</div> : filtered.map(node => <label key={node.nodeKey} className="flex items-start gap-3 border-b border-[var(--color-border)]/50 px-4 py-3 last:border-b-0 hover:bg-[var(--color-bg-hover)]"><input type="checkbox" checked={selectedSet.has(node.nodeKey)} onChange={e => toggleNode(node.nodeKey, e.target.checked)} className="mt-1" /><div className="min-w-0 flex-1"><div className="flex flex-wrap items-center gap-2"><div className="truncate text-sm font-medium text-[var(--color-text-primary)]">{node.nodeName}</div><Badge size="sm">{node.displayGroup || '未分组'}</Badge><Badge size="sm">{(node.protocol || 'unknown').toUpperCase()}</Badge>{node.chainMode === 'chained' && <Badge size="sm" variant="info">链式</Badge>}</div><div className="mt-1 flex flex-wrap gap-3 text-xs text-[var(--color-text-muted)]"><span>{node.server || '-'}</span><span>{node.port || '-'}</span>{node.upstreamAlias && <span>上游：{node.upstreamAlias}</span>}</div></div></label>)}</div></div></Modal>
}
