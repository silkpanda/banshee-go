const MAX_TOKEN_LENGTH = 4096

export function validateRelayUrl(raw: string): string | null {
  const trimmed = raw.trim()
  if (trimmed.length === 0) {
    return 'Relay URL cannot be empty.'
  }
  let parsed: URL
  try {
    parsed = new URL(trimmed)
  } catch {
    return 'Enter a valid https:// URL.'
  }
  if (parsed.protocol !== 'https:') {
    return 'Relay URL must use https://.'
  }
  if (parsed.hostname.length === 0) {
    return 'Relay URL must include a hostname.'
  }
  return null
}

export function validateSecretToken(raw: string): string | null {
  const trimmed = raw.trim()
  if (trimmed.length === 0) {
    return 'Token cannot be empty.'
  }
  if (trimmed.length > MAX_TOKEN_LENGTH) {
    return `Token exceeds ${MAX_TOKEN_LENGTH} characters.`
  }
  return null
}
