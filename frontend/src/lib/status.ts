export type RelayState = 'connected' | 'degraded' | 'offline' | 'standby'

export interface RelayBadgeInfo {
  label: string
  variant: 'default' | 'secondary' | 'destructive' | 'outline'
}

const RELAY_STATE_MAP: Record<RelayState, RelayBadgeInfo> = {
  connected: { label: 'Connected', variant: 'default' },
  degraded: { label: 'Degraded', variant: 'secondary' },
  offline: { label: 'Offline', variant: 'destructive' },
  standby: { label: 'Standby', variant: 'outline' }
}

const RELAY_TONE_CLASSES: Record<RelayState, string> = {
  connected: 'border-healthy/30 bg-healthy/15 text-healthy',
  degraded: 'border-degraded/30 bg-degraded/15 text-degraded',
  offline: 'border-destructive/30 bg-destructive/15 text-destructive',
  standby: 'border-border bg-muted text-muted-foreground'
}

const RELAY_DOT_CLASSES: Record<RelayState, string> = {
  connected: 'bg-healthy',
  degraded: 'bg-degraded',
  offline: 'bg-destructive',
  standby: 'bg-muted-foreground'
}

export function normalizeRelayState(state: string): RelayState {
  if (state in RELAY_STATE_MAP) {
    return state as RelayState
  }
  return 'standby'
}

export function describeRelayState(state: string): RelayBadgeInfo {
  return RELAY_STATE_MAP[normalizeRelayState(state)]
}

export function relayBadgeClassName(state: string): string {
  return RELAY_TONE_CLASSES[normalizeRelayState(state)]
}

export function relayDotClassName(state: string): string {
  return RELAY_DOT_CLASSES[normalizeRelayState(state)]
}

export function formatContactTime(iso?: string): string {
  if (!iso) return 'Never'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return 'Never'
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

export function describeTokenSource(source?: string): string {
  switch (source) {
    case 'dpapi':
      return 'Windows DPAPI'
    case 'env':
      return 'Local .env fallback'
    default:
      return 'Not configured'
  }
}
