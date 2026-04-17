import { useEffect, useMemo, useState } from 'react'
import { Button, FormItem, Input, Modal, Switch } from '../../../shared/components'
import type { BrowserSubscriptionSource } from '../types'

interface SubscriptionSourceModalProps {
  open: boolean
  source: BrowserSubscriptionSource | null
  saving: boolean
  onClose: () => void
  onSubmit: (source: BrowserSubscriptionSource) => Promise<void>
}

const createDefaultSource = (): BrowserSubscriptionSource => ({
  sourceId: '',
  name: '',
  url: '',
  enabled: true,
  refreshIntervalMinutes: 60,
  lastRefreshAt: '',
  lastRefreshStatus: '',
  lastError: '',
  trafficUsed: '',
  trafficTotal: '',
  expireAt: '',
})

export function SubscriptionSourceModal({ open, source, saving, onClose, onSubmit }: SubscriptionSourceModalProps) {
  const [form, setForm] = useState<BrowserSubscriptionSource>(createDefaultSource())
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) return
    setForm(source ? { ...source } : createDefaultSource())
    setError('')
  }, [open, source])

  const title = useMemo(() => (form.sourceId ? '编辑订阅' : '新增订阅'), [form.sourceId])

  const handleSubmit = async () => {
    const name = form.name.trim()
    const url = form.url.trim()
    if (!name || !url) {
      setError('请填写订阅名称和订阅 URL')
      return
    }
    setError('')
    await onSubmit({ ...form, name, url, refreshIntervalMinutes: Math.max(1, Number(form.refreshIntervalMinutes || 60)) })
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      width="640px"
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={onClose}>取消</Button>
          <Button onClick={handleSubmit} loading={saving}>保存</Button>
        </div>
      }
    >
      <div className="space-y-4">
        <FormItem label="订阅名称" required error={error && !form.name.trim() ? error : undefined}>
          <Input value={form.name} onChange={e => setForm(prev => ({ ...prev, name: e.target.value }))} placeholder="例如：Residential IDC US2" />
        </FormItem>
        <FormItem label="订阅 URL" required error={error && !form.url.trim() ? error : undefined}>
          <Input value={form.url} onChange={e => setForm(prev => ({ ...prev, url: e.target.value }))} placeholder="https://example.com/subscription" />
        </FormItem>
        <div className="grid gap-4 md:grid-cols-2">
          <FormItem label="刷新间隔（分钟)">
            <Input type="number" min={1} value={String(form.refreshIntervalMinutes || 60)} onChange={e => setForm(prev => ({ ...prev, refreshIntervalMinutes: Number(e.target.value || 60) }))} />
          </FormItem>
          <FormItem label="启用状态">
            <div className="flex h-9 items-center"><Switch checked={!!form.enabled} onChange={checked => setForm(prev => ({ ...prev, enabled: checked }))} /></div>
          </FormItem>
        </div>
      </div>
    </Modal>
  )
}
