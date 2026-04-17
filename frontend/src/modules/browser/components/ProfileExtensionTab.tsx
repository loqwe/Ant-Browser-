import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { Badge, Button, Card, Switch, toast } from '../../../shared/components'
import { fetchBrowserExtensions, fetchBrowserProfileExtensions, saveBrowserProfileExtensions } from '../api'
import type { BrowserExtension, BrowserProfileExtensionBinding } from '../types'

export function ProfileExtensionTab({ profileId, running }: { profileId: string; running: boolean }) {
  const [extensions, setExtensions] = useState<BrowserExtension[]>([])
  const [bindings, setBindings] = useState<BrowserProfileExtensionBinding[]>([])

  const loadData = async () => {
    const [allExtensions, allBindings] = await Promise.all([
      fetchBrowserExtensions(),
      fetchBrowserProfileExtensions(profileId),
    ])
    setExtensions(allExtensions)
    setBindings(allBindings)
  }

  useEffect(() => { void loadData() }, [profileId])

  const boundMap = useMemo(() => {
    const map = new Map<string, BrowserProfileExtensionBinding>()
    bindings.forEach(item => map.set(item.extensionId, item))
    return map
  }, [bindings])

  const globalExtensions = useMemo(() => extensions.filter(item => item.enabledByDefault), [extensions])
  const localExtensions = useMemo(() => extensions.filter(item => !item.enabledByDefault), [extensions])

  const persist = async (next: BrowserProfileExtensionBinding[], message: string) => {
    await saveBrowserProfileExtensions(profileId, next)
    setBindings(next)
    toast.success(message)
  }

  const handleBindToggle = async (item: BrowserExtension) => {
    const current = boundMap.get(item.extensionId)
    if (current) {
      await persist(bindings.filter(binding => binding.extensionId !== item.extensionId), `已解绑：${item.name}`)
      return
    }
    const nextItem: BrowserProfileExtensionBinding = {
      bindingId: `bind-${Date.now()}-${item.extensionId}`,
      profileId,
      extensionId: item.extensionId,
      enabled: true,
      sortOrder: bindings.length + 1,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    }
    await persist([...bindings, nextItem], `已绑定：${item.name}`)
  }

  const handleEnabledChange = async (item: BrowserExtension, checked: boolean) => {
    const next = bindings.map(binding =>
      binding.extensionId === item.extensionId
        ? { ...binding, enabled: checked, updatedAt: new Date().toISOString() }
        : binding
    )
    await persist(next, checked ? `已启用：${item.name}` : `已禁用：${item.name}`)
  }

  return (
    <Card title="实例扩展" subtitle={running ? '全局扩展在实例重启后生效；本地扩展绑定修改也需重启实例' : 'CRX 扩展已对全部实例默认生效，本地扩展可按实例单独绑定'}>
      <div className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-3 text-sm">
          <div className="flex flex-wrap items-center gap-2 text-[var(--color-text-secondary)]">
            <span>全局扩展 {globalExtensions.length} 项</span>
            <span>·</span>
            <span>实例绑定 {bindings.length} 项</span>
          </div>
          <Link to="/browser/extensions">
            <Button size="sm" variant="secondary">前往扩展仓库</Button>
          </Link>
        </div>

        {globalExtensions.length > 0 && (
          <div className="space-y-3">
            <div className="text-sm font-medium text-[var(--color-text-primary)]">全局扩展</div>
            {globalExtensions.map(item => (
              <div key={item.extensionId} className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="space-y-1 min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <div className="font-medium text-[var(--color-text-primary)]">{item.name}</div>
                      <Badge variant="success">全局生效</Badge>
                    </div>
                    <div className="text-xs text-[var(--color-text-muted)]">{item.version || '未知版本'} · {item.sourceType}</div>
                    <div className="text-sm text-[var(--color-text-secondary)]">{item.description || '暂无描述'}</div>
                  </div>
                  <div className="text-xs text-[var(--color-text-muted)] whitespace-nowrap">{running ? '重启后加载' : '启动即加载'}</div>
                </div>
              </div>
            ))}
          </div>
        )}

        <div className="space-y-3">
          <div className="text-sm font-medium text-[var(--color-text-primary)]">实例绑定扩展</div>
          {localExtensions.map(item => {
            const binding = boundMap.get(item.extensionId)
            return (
              <div key={item.extensionId} className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="space-y-1 min-w-0">
                    <div className="font-medium text-[var(--color-text-primary)]">{item.name}</div>
                    <div className="text-xs text-[var(--color-text-muted)]">{item.version || '未知版本'} · {item.sourceType}</div>
                    <div className="text-sm text-[var(--color-text-secondary)]">{item.description || '暂无描述'}</div>
                  </div>
                  <Button size="sm" variant={binding ? 'secondary' : 'primary'} onClick={() => handleBindToggle(item)}>
                    {binding ? '解绑' : '绑定'}
                  </Button>
                </div>
                {binding && (
                  <div className="mt-3 flex items-center justify-between rounded-md border border-[var(--color-border)] px-3 py-2">
                    <span className="text-sm text-[var(--color-text-secondary)]">启用状态</span>
                    <Switch checked={binding.enabled} onChange={(checked) => handleEnabledChange(item, checked)} />
                  </div>
                )}
              </div>
            )
          })}
          {localExtensions.length === 0 && (
            <div className="rounded-lg border border-dashed border-[var(--color-border)] p-8 text-center text-sm text-[var(--color-text-muted)]">
              当前没有需要单独绑定的本地扩展
            </div>
          )}
        </div>

        {extensions.length === 0 && (
          <div className="rounded-lg border border-dashed border-[var(--color-border)] p-8 text-center text-sm text-[var(--color-text-muted)]">
            扩展仓库为空，请先导入扩展
          </div>
        )}
      </div>
    </Card>
  )
}