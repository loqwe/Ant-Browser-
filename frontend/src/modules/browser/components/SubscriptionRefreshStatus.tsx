import { Badge } from '../../../shared/components'
import type { SubscriptionRefreshState } from '../types'

interface SubscriptionRefreshStatusProps {
  state?: SubscriptionRefreshState
}

const stageConfig = {
  requested: { variant: 'info' as const, label: '???????' },
  backendSynced: { variant: 'info' as const, label: '????????????' },
  uiSynced: { variant: 'success' as const, label: '?????' },
  failed: { variant: 'error' as const, label: '' },
}

export function SubscriptionRefreshStatus({ state }: SubscriptionRefreshStatusProps) {
  if (!state) return null
  const config = stageConfig[state.stage]
  const message = state.stage === 'failed' ? state.message : (state.message || config.label)
  return (
    <div className="mt-3">
      <Badge variant={config.variant} size="sm" dot className="max-w-full align-middle">
        <span className="truncate">{message}</span>
      </Badge>
    </div>
  )
}
