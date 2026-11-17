import { useState } from 'react'
import { KeyRound, Save, Trash2, Volume2 } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle
} from '@/components/ui/card'
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { validateRelayUrl, validateSecretToken } from '@/lib/validation'
import type { secrets, store } from '../../wailsjs/go/models'

interface SettingsPanelProps {
  settings: store.SettingsSnapshot
  secretsStatus: secrets.SecretStatus
  onUpdateSetting: (key: string, value: unknown) => Promise<void>
  onSaveSecret: (name: string, plaintext: string) => Promise<void>
  onClearSecret: (name: string) => Promise<void>
}

export function SettingsPanel({
  settings,
  secretsStatus,
  onUpdateSetting,
  onSaveSecret,
  onClearSecret
}: SettingsPanelProps) {
  const [relayDraft, setRelayDraft] = useState(settings.relayUrl)
  const [relayError, setRelayError] = useState<string | null>(null)
  const [deviceTokenDraft, setDeviceTokenDraft] = useState('')
  const [turingTokenDraft, setTuringTokenDraft] = useState('')
  const [tokenError, setTokenError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const handleSaveRelay = async () => {
    const err = validateRelayUrl(relayDraft)
    setRelayError(err)
    if (err !== null) return

    setBusy(true)
    setNotice(null)
    try {
      await onUpdateSetting('relay_url', relayDraft.trim())
      setNotice('Relay URL saved to SQLite.')
    } catch (e: unknown) {
      setRelayError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  const handleSaveToken = async (name: 'device_token' | 'turing_token', value: string) => {
    const err = validateSecretToken(value)
    setTokenError(err)
    if (err !== null) return

    setBusy(true)
    setNotice(null)
    try {
      await onSaveSecret(name, value)
      if (name === 'device_token') setDeviceTokenDraft('')
      if (name === 'turing_token') setTuringTokenDraft('')
      setNotice(`Encrypted ${name} stored via Windows DPAPI.`)
    } catch (e: unknown) {
      setTokenError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  const handleClearToken = async (name: 'device_token' | 'turing_token') => {
    setBusy(true)
    setNotice(null)
    setTokenError(null)
    try {
      await onClearSecret(name)
      setNotice(`Cleared ${name} from encrypted store.`)
    } catch (e: unknown) {
      setTokenError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="grid gap-4 md:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Volume2 />
            Relay &amp; Alert Preferences
          </CardTitle>
          <CardDescription>
            Allowlisted settings persisted locally in SQLite
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <FieldGroup>
            <Field data-invalid={relayError !== null ? true : undefined}>
              <FieldLabel htmlFor="relay-url-input">Relay URL</FieldLabel>
              <div className="flex items-center gap-2">
                <Input
                  id="relay-url-input"
                  value={relayDraft}
                  onChange={(e) => setRelayDraft(e.target.value)}
                  aria-invalid={relayError !== null ? true : undefined}
                  placeholder="https://relay.example.com"
                  className="font-mono text-xs"
                />
                <Button
                  size="sm"
                  disabled={busy}
                  onClick={() => void handleSaveRelay()}
                >
                  <Save data-icon="inline-start" />
                  Save
                </Button>
              </div>
              <FieldDescription>
                {relayError ?? 'Requires https:// — WebSocket wss:// is derived from this.'}
              </FieldDescription>
            </Field>

            <Field>
              <FieldLabel>Event Filter</FieldLabel>
              <ToggleGroup
                value={[settings.eventTypeFilter]}
                variant="outline"
                size="sm"
                onValueChange={(next) => {
                  const chosen = next[0]
                  if (chosen) {
                    void onUpdateSetting('event_type_filter', chosen)
                  }
                }}
              >
                <ToggleGroupItem value="all">All</ToggleGroupItem>
                <ToggleGroupItem value="people">People</ToggleGroupItem>
                <ToggleGroupItem value="vehicle">Vehicle</ToggleGroupItem>
              </ToggleGroup>
              <FieldDescription>
                Choose which Turing AI alert categories trigger the siren.
              </FieldDescription>
            </Field>

            <Separator />

            <div className="flex items-center justify-between gap-4">
              <div className="flex flex-col gap-0.5">
                <span className="text-sm font-medium">Mute Banshee Siren</span>
                <span className="text-xs text-muted-foreground">
                  Visual alert cards remain active while audio is muted
                </span>
              </div>
              <Switch
                checked={settings.soundMuted}
                disabled={busy}
                onCheckedChange={(checked) => void onUpdateSetting('sound_muted', checked)}
              />
            </div>
          </FieldGroup>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <KeyRound />
            DPAPI Credential Vault
          </CardTitle>
          <CardDescription>
            Plaintext tokens are never logged or returned to the UI
          </CardDescription>
          <CardAction>
            <Badge variant={secretsStatus.encryptionAvailable ? 'default' : 'destructive'}>
              {secretsStatus.encryptionAvailable ? 'DPAPI Active' : 'DPAPI Unavailable'}
            </Badge>
          </CardAction>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <FieldGroup>
            <Field>
              <div className="flex items-center justify-between">
                <FieldLabel htmlFor="device-token-input">Relay Device Token</FieldLabel>
                <Badge variant={secretsStatus.deviceTokenSet ? 'default' : 'outline'}>
                  {secretsStatus.deviceTokenSet ? 'Stored' : 'Not set'}
                </Badge>
              </div>
              <div className="flex items-center gap-2">
                <Input
                  id="device-token-input"
                  type="password"
                  value={deviceTokenDraft}
                  onChange={(e) => setDeviceTokenDraft(e.target.value)}
                  placeholder="Paste relay device bearer token"
                  className="font-mono text-xs"
                />
                <Button
                  size="sm"
                  disabled={busy || !secretsStatus.encryptionAvailable}
                  onClick={() => void handleSaveToken('device_token', deviceTokenDraft)}
                >
                  Save
                </Button>
                {secretsStatus.deviceTokenSet && (
                  <Button
                    size="icon-sm"
                    variant="outline"
                    disabled={busy}
                    aria-label="Clear device token"
                    onClick={() => void handleClearToken('device_token')}
                  >
                    <Trash2 />
                  </Button>
                )}
              </div>
            </Field>

            <Field>
              <div className="flex items-center justify-between">
                <FieldLabel htmlFor="turing-token-input">Turing Vision Token</FieldLabel>
                <Badge variant={secretsStatus.turingTokenSet ? 'default' : 'outline'}>
                  {secretsStatus.turingTokenSet ? 'Stored' : 'Not set'}
                </Badge>
              </div>
              <div className="flex items-center gap-2">
                <Input
                  id="turing-token-input"
                  type="password"
                  value={turingTokenDraft}
                  onChange={(e) => setTuringTokenDraft(e.target.value)}
                  placeholder="Paste Turing OpenAPI access token"
                  className="font-mono text-xs"
                />
                <Button
                  size="sm"
                  disabled={busy || !secretsStatus.encryptionAvailable}
                  onClick={() => void handleSaveToken('turing_token', turingTokenDraft)}
                >
                  Save
                </Button>
                {secretsStatus.turingTokenSet && (
                  <Button
                    size="icon-sm"
                    variant="outline"
                    disabled={busy}
                    aria-label="Clear Turing token"
                    onClick={() => void handleClearToken('turing_token')}
                  >
                    <Trash2 />
                  </Button>
                )}
              </div>
            </Field>
          </FieldGroup>

          {tokenError !== null && (
            <Alert variant="destructive">
              <AlertTitle>Credential Error</AlertTitle>
              <AlertDescription>{tokenError}</AlertDescription>
            </Alert>
          )}
          {notice !== null && tokenError === null && (
            <Alert>
              <AlertTitle>Saved</AlertTitle>
              <AlertDescription>{notice}</AlertDescription>
            </Alert>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
