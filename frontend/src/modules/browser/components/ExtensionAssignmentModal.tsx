import { useEffect, useMemo, useState, type Dispatch, type SetStateAction } from 'react'
import { Badge, Button, Input, Modal, toast } from '../../../shared/components'
import { fetchBrowserProfileExtensions, fetchBrowserProfiles, fetchGroups, saveBrowserProfileExtensions } from '../api'
import type { BrowserExtension, BrowserGroupWithCount, BrowserProfile, BrowserProfileExtensionBinding } from '../types'

type AssignMode = 'profiles' | 'groups' | 'tags'

interface Props {
  open: boolean
  extension: BrowserExtension | null
  onClose: () => void
  onSaved: () => Promise<void>
}

export function ExtensionAssignmentModal({ open, extension, onClose, onSaved }: Props) {
  const [mode, setMode] = useState<AssignMode>('profiles')
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [groups, setGroups] = useState<BrowserGroupWithCount[]>([])
  const [selectedProfiles, setSelectedProfiles] = useState<Set<string>>(new Set())
  const [selectedGroups, setSelectedGroups] = useState<Set<string>>(new Set())
  const [selectedTags, setSelectedTags] = useState<Set<string>>(new Set())
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [bindingsMap, setBindingsMap] = useState<Record<string, BrowserProfileExtensionBinding[]>>({})

  useEffect(() => {
    if (!open || !extension) return
    const load = async () => {
      setLoading(true)
      try {
        const profileList = await fetchBrowserProfiles()
        const groupList = await fetchGroups()
        const bindingEntries = await Promise.all(profileList.map(async profile => [profile.profileId, await fetchBrowserProfileExtensions(profile.profileId)] as const))
        const nextMap: Record<string, BrowserProfileExtensionBinding[]> = {}
        const assigned = new Set<string>()
        bindingEntries.forEach(([profileId, bindings]) => {
          nextMap[profileId] = bindings
          if (bindings.some(item => item.extensionId === extension.extensionId && item.enabled)) assigned.add(profileId)
        })
        setProfiles(profileList)
        setGroups(groupList)
        setBindingsMap(nextMap)
        setSelectedProfiles(assigned)
        setSelectedGroups(new Set())
        setSelectedTags(new Set())
        setSearch('')
        setMode('profiles')
      } finally {
        setLoading(false)
      }
    }
    void load()
  }, [open, extension])

  const allTags = useMemo(() => Array.from(new Set(profiles.flatMap(profile => profile.tags || []))).sort(), [profiles])
  const resolvedProfileIds = useMemo(() => {
    const result = new Set(selectedProfiles)
    profiles.forEach(profile => {
      if (profile.groupId && selectedGroups.has(profile.groupId)) result.add(profile.profileId)
      if ((profile.tags || []).some(tag => selectedTags.has(tag))) result.add(profile.profileId)
    })
    return result
  }, [profiles, selectedGroups, selectedProfiles, selectedTags])
  const resolvedProfiles = useMemo(() => profiles.filter(profile => resolvedProfileIds.has(profile.profileId)), [profiles, resolvedProfileIds])
  const keyword = search.trim().toLowerCase()
  const visibleProfiles = useMemo(() => profiles.filter(profile => !keyword || [profile.profileName, ...(profile.tags || [])].join(' ').toLowerCase().includes(keyword)), [profiles, keyword])
  const visibleGroups = useMemo(() => groups.filter(group => !keyword || group.groupName.toLowerCase().includes(keyword)), [groups, keyword])
  const visibleTags = useMemo(() => allTags.filter(tag => !keyword || tag.toLowerCase().includes(keyword)), [allTags, keyword])

  const toggle = (setValue: Dispatch<SetStateAction<Set<string>>>, value: string) => setValue(prev => {
    const next = new Set(prev)
    next.has(value) ? next.delete(value) : next.add(value)
    return next
  })

  const handleSave = async () => {
    if (!extension) return
    setSaving(true)
    try {
      for (const profile of profiles) {
        const current = bindingsMap[profile.profileId] || []
        const existing = current.find(item => item.extensionId === extension.extensionId)
        const shouldHave = resolvedProfileIds.has(profile.profileId)
        let next = current
        if (shouldHave && !existing) {
          next = [...current, { bindingId: `bind-${Date.now()}-${profile.profileId}`, profileId: profile.profileId, extensionId: extension.extensionId, enabled: true, sortOrder: current.length + 1, createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() }]
        } else if (shouldHave && existing && !existing.enabled) {
          next = current.map(item => item.extensionId === extension.extensionId ? { ...item, enabled: true, updatedAt: new Date().toISOString() } : item)
        } else if (!shouldHave && existing) {
          next = current.filter(item => item.extensionId !== extension.extensionId)
        }
        if (next !== current) await saveBrowserProfileExtensions(profile.profileId, next)
      }
      toast.success('扩展分配已保存')
      await onSaved()
      onClose()
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={`分配环境（${extension?.name || ''}）`} width="980px" footer={<><Button variant="secondary" onClick={onClose} disabled={saving}>取消</Button><Button onClick={() => void handleSave()} loading={saving}>确定</Button></>}>
      <div className="space-y-4">
        <div className="flex items-center gap-3 border-b border-[var(--color-border-default)] pb-3 text-sm">
          {(['profiles', 'groups', 'tags'] as const).map(item => <button key={item} type="button" onClick={() => setMode(item)} className={`rounded-lg px-3 py-1.5 ${mode === item ? 'bg-[var(--color-accent)]/10 text-[var(--color-accent)]' : 'text-[var(--color-text-secondary)]'}`}>{item === 'profiles' ? '按实例' : item === 'groups' ? `按分组 (${groups.length})` : `按标签 (${allTags.length})`}</button>)}
        </div>
        <div className="grid min-h-[430px] gap-4 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
          <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-4">
            <Input value={search} onChange={e => setSearch(e.target.value)} placeholder={mode === 'profiles' ? '搜索实例' : mode === 'groups' ? '搜索分组' : '搜索标签'} className="mb-3" />
            <div className="max-h-[340px] space-y-2 overflow-y-auto pr-1 text-sm">
              {loading ? <div className="py-10 text-center text-[var(--color-text-muted)]">加载中...</div> : mode === 'profiles' ? visibleProfiles.map(profile => <label key={profile.profileId} className="flex items-center gap-3 rounded-lg px-2 py-2 hover:bg-[var(--color-bg-secondary)]"><input type="checkbox" checked={selectedProfiles.has(profile.profileId)} onChange={() => toggle(setSelectedProfiles, profile.profileId)} /><span className="min-w-0 flex-1 truncate text-[var(--color-text-primary)]">{profile.profileName}</span></label>) : mode === 'groups' ? visibleGroups.map(group => <label key={group.groupId} className="flex items-center gap-3 rounded-lg px-2 py-2 hover:bg-[var(--color-bg-secondary)]"><input type="checkbox" checked={selectedGroups.has(group.groupId)} onChange={() => toggle(setSelectedGroups, group.groupId)} /><span className="min-w-0 flex-1 truncate text-[var(--color-text-primary)]">{group.groupName}</span><Badge size="sm">{group.instanceCount}</Badge></label>) : visibleTags.map(tag => <label key={tag} className="flex items-center gap-3 rounded-lg px-2 py-2 hover:bg-[var(--color-bg-secondary)]"><input type="checkbox" checked={selectedTags.has(tag)} onChange={() => toggle(setSelectedTags, tag)} /><span className="min-w-0 flex-1 truncate text-[var(--color-text-primary)]">{tag}</span></label>)}
            </div>
          </div>
          <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-4">
            <div className="mb-3 flex items-center justify-between text-sm"><span className="text-[var(--color-text-secondary)]">已选环境</span><button type="button" className="text-[var(--color-accent)]" onClick={() => { setSelectedProfiles(new Set()); setSelectedGroups(new Set()); setSelectedTags(new Set()) }}>清空已选</button></div>
            <div className="max-h-[340px] space-y-2 overflow-y-auto pr-1 text-sm">
              {resolvedProfiles.length === 0 ? <div className="py-20 text-center text-[var(--color-text-muted)]">暂无数据</div> : resolvedProfiles.map(profile => <div key={profile.profileId} className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)]/50 px-3 py-2"><div className="text-[var(--color-text-primary)]">{profile.profileName}</div><div className="mt-1 flex flex-wrap gap-1">{profile.groupId ? <Badge size="sm">分组</Badge> : null}{(profile.tags || []).slice(0, 3).map(tag => <Badge size="sm" key={tag}>{tag}</Badge>)}</div></div>)}
            </div>
          </div>
        </div>
      </div>
    </Modal>
  )
}