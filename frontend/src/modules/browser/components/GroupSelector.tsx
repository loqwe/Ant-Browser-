import { useMemo } from 'react'
import type { BrowserGroup } from '../types'

interface GroupSelectorProps {
  groups: BrowserGroup[]
  value: string
  onChange: (groupId: string) => void
  placeholder?: string
  className?: string
}

interface FlatGroup extends BrowserGroup {
  level: number
}

function normalizeParentId(parentId?: string | null) {
  return (parentId || '').trim()
}

// 将分组列表扁平化并计算层级
function flattenGroups(groups: BrowserGroup[]): FlatGroup[] {
  const result: FlatGroup[] = []

  const addChildren = (parentId: string, level: number) => {
    groups
      .filter(g => normalizeParentId(g.parentId) === parentId)
      .sort((a, b) => a.sortOrder - b.sortOrder)
      .forEach(g => {
        result.push({ ...g, level })
        addChildren(g.groupId, level + 1)
      })
  }

  groups
    .filter(g => !normalizeParentId(g.parentId))
    .sort((a, b) => a.sortOrder - b.sortOrder)
    .forEach(g => {
      result.push({ ...g, level: 0 })
      addChildren(g.groupId, 1)
    })

  return result
}

export function GroupSelector({ groups, value, onChange, placeholder = '选择分组', className = '' }: GroupSelectorProps) {
  const flatGroups = useMemo(() => flattenGroups(groups), [groups])

  return (
    <select
      className={`px-3 py-2 border rounded dark:bg-gray-700 dark:border-gray-600 ${className}`}
      value={value}
      onChange={e => onChange(e.target.value)}
    >
      <option value="">{placeholder}</option>
      {flatGroups.map(g => (
        <option key={g.groupId} value={g.groupId}>
          {'　'.repeat(g.level)}{g.groupName}
        </option>
      ))}
    </select>
  )
}
