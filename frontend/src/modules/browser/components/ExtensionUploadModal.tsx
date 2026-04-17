import { useEffect, useMemo, useState } from 'react'
import { FolderOpen, Paperclip, Store, Upload } from 'lucide-react'
import { Button, Input, Modal } from '../../../shared/components'

type UploadSource = 'store' | 'custom'
type CustomMode = 'package' | 'dir'

interface ExtensionUploadModalProps {
  open: boolean
  submitting: boolean
  onClose: () => void
  onPickPackage: () => Promise<string>
  onPickDir: () => Promise<string>
  onSubmitStore: (extensionId: string) => Promise<void>
  onSubmitCustom: (mode: CustomMode, path: string) => Promise<void>
}

function parseChromeExtensionId(value: string) {
  const input = value.trim().toLowerCase()
  if (!input) return ''
  if (/^[a-z]{32}$/.test(input)) return input
  const fromDetail = input.match(/\/detail\/[^/]+\/([a-z]{32})(?:[/?#]|$)/i)
  if (fromDetail?.[1]) return fromDetail[1].toLowerCase()
  const fromQuery = input.match(/[?&]id=([a-z]{32})(?:[&#]|$)/i)
  if (fromQuery?.[1]) return fromQuery[1].toLowerCase()
  return ''
}

function getPathLeaf(path: string) {
  return path.split(/[\\/]/).filter(Boolean).pop() || path
}

export function ExtensionUploadModal({ open, submitting, onClose, onPickPackage, onPickDir, onSubmitStore, onSubmitCustom }: ExtensionUploadModalProps) {
  const [source, setSource] = useState<UploadSource>('custom')
  const [storeInput, setStoreInput] = useState('')
  const [selectedPath, setSelectedPath] = useState('')
  const [selectedMode, setSelectedMode] = useState<CustomMode | null>(null)
  const [selecting, setSelecting] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) return
    setSource('custom')
    setStoreInput('')
    setSelectedPath('')
    setSelectedMode(null)
    setSelecting(false)
    setError('')
  }, [open])

  const parsedExtensionId = useMemo(() => parseChromeExtensionId(storeInput), [storeInput])
  const canSubmit = source === 'store' ? !!parsedExtensionId : !!selectedPath && !!selectedMode
  const selectedName = useMemo(() => selectedPath ? getPathLeaf(selectedPath) : '', [selectedPath])

  const handlePickPackage = async () => {
    setError('')
    setSelecting(true)
    try {
      const path = await onPickPackage()
      if (path) {
        setSelectedPath(path)
        setSelectedMode('package')
      }
    } catch (error: any) {
      setError(error?.message || '选择扩展文件失败')
    } finally {
      setSelecting(false)
    }
  }

  const handlePickDir = async () => {
    setError('')
    setSelecting(true)
    try {
      const path = await onPickDir()
      if (path) {
        setSelectedPath(path)
        setSelectedMode('dir')
      }
    } catch (error: any) {
      setError(error?.message || '选择扩展目录失败')
    } finally {
      setSelecting(false)
    }
  }

  const handleSubmit = async () => {
    if (source === 'store') {
      if (!parsedExtensionId) {
        setError('请输入正确的扩展 ID 或 Chrome 应用商店链接')
        return
      }
      setError('')
      await onSubmitStore(parsedExtensionId)
      return
    }
    if (!selectedPath || !selectedMode) {
      setError('请先选择扩展文件或扩展目录')
      return
    }
    setError('')
    await onSubmitCustom(selectedMode, selectedPath)
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="上传扩展"
      width="760px"
      footer={<><Button variant="secondary" onClick={onClose} disabled={submitting || selecting}>取消</Button><Button onClick={() => void handleSubmit()} loading={submitting} disabled={!canSubmit || selecting}>确定</Button></>}
    >
      <div className="space-y-6">
        <div className="flex items-center gap-3 text-sm"><span className="text-red-500">*</span><span className="w-24 shrink-0 text-[var(--color-text-secondary)]">扩展类型：</span><label className="inline-flex items-center gap-2 text-[15px] text-[var(--color-text-primary)]"><input type="radio" checked readOnly className="accent-[var(--color-accent)]" /><span>谷歌扩展</span></label></div>

        <div className="flex items-center gap-3 text-sm"><span className="text-red-500">*</span><span className="w-24 shrink-0 text-[var(--color-text-secondary)]">扩展来源：</span><label className="inline-flex items-center gap-2 text-[15px] text-[var(--color-text-primary)]"><input type="radio" checked={source === 'store'} onChange={() => { setSource('store'); setError('') }} className="accent-[var(--color-accent)]" /><span>Chrome 应用商店</span></label><label className="inline-flex items-center gap-2 text-[15px] text-[var(--color-text-primary)]"><input type="radio" checked={source === 'custom'} onChange={() => { setSource('custom'); setError('') }} className="accent-[var(--color-accent)]" /><span>自建扩展</span></label></div>

        {source === 'store' ? (
          <div className="space-y-3 rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-4">
            <div className="flex items-center gap-2 text-[15px] font-medium text-[var(--color-text-primary)]"><Store className="h-4 w-4 text-[var(--color-accent)]" />商店扩展</div>
            <div className="grid gap-3 md:grid-cols-[104px_minmax(0,1fr)] md:items-center">
              <div className="text-sm text-[var(--color-text-secondary)]">扩展标识：</div>
              <Input value={storeInput} onChange={e => setStoreInput(e.target.value)} placeholder="请输入扩展 ID 或 Chrome 应用商店链接" />
            </div>
            <div className="pl-0 text-xs text-[var(--color-text-muted)] md:pl-[116px]">示例：`aapbdbdomjkkjkaonfhkkikfgjllcleb` 或完整商店链接</div>
          </div>
        ) : (
          <div className="space-y-4 rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-4">
            <div className="grid gap-3 md:grid-cols-[104px_minmax(0,1fr)] md:items-center">
              <div className="text-sm text-[var(--color-text-secondary)]">安装文件：</div>
              <div className="flex flex-wrap items-center gap-3">
                <Button type="button" variant="secondary" onClick={() => void handlePickPackage()} loading={selecting}><Upload className="h-4 w-4" />上传文件</Button>
                <button type="button" onClick={() => void handlePickDir()} disabled={selecting} className="inline-flex items-center gap-2 text-sm text-[var(--color-accent)] transition-colors hover:opacity-80 disabled:cursor-not-allowed disabled:opacity-50"><FolderOpen className="h-4 w-4" />选择扩展目录</button>
                <span className="text-xs text-[var(--color-text-muted)]">支持扩展名：`.crx`、`.zip`（自动解析）</span>
              </div>
            </div>
            {selectedPath && (
              <div className="grid gap-3 md:grid-cols-[104px_minmax(0,1fr)] md:items-center">
                <div className="text-sm text-[var(--color-text-secondary)]">已选文件：</div>
                <div className="flex flex-wrap items-center gap-2 text-sm text-[var(--color-text-secondary)]">
                  <div title={selectedPath} className="inline-flex max-w-full items-center gap-2 rounded-md border border-[var(--color-border-default)]/70 bg-[var(--color-bg-secondary)]/55 px-2.5 py-1.5 text-[13px] text-[var(--color-text-secondary)] shadow-none">
                    <Paperclip className="h-3.5 w-3.5 shrink-0 text-[var(--color-text-muted)]" />
                    <span className="max-w-[320px] truncate text-[var(--color-text-primary)]/85">{selectedName}</span>
                  </div>
                  <span className="text-[12px] text-[var(--color-text-muted)]">{selectedMode === 'package' ? '扩展文件' : '扩展目录'}</span>
                </div>
              </div>
            )}
          </div>
        )}

        {error && <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-600">{error}</div>}
      </div>
    </Modal>
  )
}