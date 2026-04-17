import type { BrowserProxy } from './types'

export interface SubscriptionProxyGroup {
  name: string
  type?: string
  proxies?: string[]
}

type SelectorScore = {
  priority: number
  latency: number
  hit: boolean
}

export function parseSubscriptionProxyGroups(raw?: string): SubscriptionProxyGroup[] {
  if (!raw?.trim()) return []
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

export function parseSelectedProxyGroupMap(raw?: string): Record<string, string> {
  if (!raw?.trim()) return {}
  try {
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

export function resolveRecommendedUpstream(group: SubscriptionProxyGroup, sourceProxies: BrowserProxy[]): string {
  const groupName = group.name.trim()
  const memberById = Object.fromEntries(sourceProxies.map(proxy => [proxy.proxyId, proxy]))
  const memberScores: Record<string, SelectorScore> = {}
  sourceProxies.forEach(proxy => {
    const name = resolveProxyLabel(proxy)
    if (!name) return
    const score = buildSelectorScore(proxy)
    if (!memberScores[name] || isBetterSelectorScore(score, memberScores[name])) memberScores[name] = score
  })
  sourceProxies.forEach(proxy => {
    if ((proxy.upstreamAlias || '').trim() !== groupName) return
    const member = memberById[(proxy.upstreamProxyId || '').trim()]
    if (!member) return
    const name = resolveProxyLabel(member)
    if (!name) return
    const score = buildSelectorScore(proxy)
    if (isBetterSelectorScore(score, memberScores[name])) memberScores[name] = score
  })
  let best = ''
  let bestScore: SelectorScore = { priority: 0, latency: 0, hit: false }
  ;(group.proxies || []).forEach(member => {
    const name = member.trim()
    if (!name) return
    const score = memberScores[name]
    if (!score?.hit) return
    if (!best || isBetterSelectorScore(score, bestScore)) {
      best = name
      bestScore = score
    }
  })
  return best || (group.proxies || []).map(item => item.trim()).find(Boolean) || ''
}

export function resolveCurrentUpstream(groupName: string, sourceProxies: BrowserProxy[], selectedMap: Record<string, string> = {}): string {
  const proxyById = Object.fromEntries(sourceProxies.map(proxy => [proxy.proxyId, proxy]))
  const resolved = sourceProxies.find(proxy => (proxy.upstreamAlias || '').trim() === groupName.trim() && (proxy.upstreamProxyId || '').trim())
  if (resolved) {
    const upstream = proxyById[(resolved.upstreamProxyId || '').trim()]
    const name = upstream ? resolveProxyLabel(upstream) : ''
    if (name) return name
  }
  return (selectedMap[groupName] || '').trim()
}

function resolveProxyLabel(proxy: BrowserProxy) {
  return (proxy.sourceNodeName || proxy.proxyName || '').trim()
}

function buildSelectorScore(proxy: BrowserProxy): SelectorScore {
  if (proxy.lastTestOk) return { priority: 2, latency: proxy.lastLatencyMs && proxy.lastLatencyMs > 0 ? proxy.lastLatencyMs : Number.MAX_SAFE_INTEGER, hit: true }
  if (healthOk(proxy.lastIPHealthJson)) return { priority: 1, latency: Number.MAX_SAFE_INTEGER, hit: true }
  return { priority: 0, latency: Number.MAX_SAFE_INTEGER, hit: false }
}

function isBetterSelectorScore(left: SelectorScore, right: SelectorScore) {
  if (!left.hit) return false
  if (!right.hit) return true
  if (left.priority !== right.priority) return left.priority > right.priority
  return left.priority === 2 && left.latency < right.latency
}

function healthOk(raw?: string) {
  if (!raw?.trim()) return false
  try {
    const parsed = JSON.parse(raw)
    return !!parsed?.ok
  } catch {
    return false
  }
}
