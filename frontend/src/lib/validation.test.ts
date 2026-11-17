import { describe, expect, it } from 'vitest'
import { validateRelayUrl, validateSecretToken } from './validation'

describe('validateRelayUrl', () => {
  it('accepts valid https URLs', () => {
    expect(validateRelayUrl('https://relay.example.com')).toBeNull()
  })

  it('rejects non-https or malformed URLs', () => {
    expect(validateRelayUrl('http://relay.example.com')).not.toBeNull()
    expect(validateRelayUrl('ws://relay.example.com')).not.toBeNull()
    expect(validateRelayUrl('not-a-url')).not.toBeNull()
    expect(validateRelayUrl('   ')).not.toBeNull()
  })
})

describe('validateSecretToken', () => {
  it('accepts valid non-empty tokens', () => {
    expect(validateSecretToken('  dev_token_123  ')).toBeNull()
  })

  it('rejects empty or oversized tokens', () => {
    expect(validateSecretToken('   ')).not.toBeNull()
    expect(validateSecretToken('a'.repeat(4097))).not.toBeNull()
  })
})
