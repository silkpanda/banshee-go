import { describe, expect, it } from 'vitest'
import {
  describeRelayState,
  describeTokenSource,
  formatContactTime,
  relayBadgeClassName,
  relayDotClassName
} from './status'

describe('describeRelayState', () => {
  it('maps known relay states to expected labels and badge variants', () => {
    expect(describeRelayState('connected')).toEqual({ label: 'Connected', variant: 'default' })
    expect(describeRelayState('degraded')).toEqual({ label: 'Degraded', variant: 'secondary' })
    expect(describeRelayState('offline')).toEqual({ label: 'Offline', variant: 'destructive' })
    expect(describeRelayState('standby')).toEqual({ label: 'Standby', variant: 'outline' })
  })

  it('falls back to standby for unknown states', () => {
    expect(describeRelayState('unexpected')).toEqual({ label: 'Standby', variant: 'outline' })
  })

  it('maps semantic token classes for badge and status dot', () => {
    expect(relayBadgeClassName('connected')).toContain('text-healthy')
    expect(relayBadgeClassName('degraded')).toContain('text-degraded')
    expect(relayBadgeClassName('offline')).toContain('text-destructive')
    expect(relayDotClassName('connected')).toBe('bg-healthy')
    expect(relayDotClassName('standby')).toBe('bg-muted-foreground')
  })

  it('formats contact timestamps and token sources safely', () => {
    expect(formatContactTime(undefined)).toBe('Never')
    expect(formatContactTime('invalid-date')).toBe('Never')
    expect(formatContactTime('2026-11-13T21:00:00Z')).not.toBe('Never')
    expect(describeTokenSource('dpapi')).toBe('Windows DPAPI')
    expect(describeTokenSource('env')).toBe('Local .env fallback')
    expect(describeTokenSource('none')).toBe('Not configured')
  })
})
