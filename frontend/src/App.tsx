import { useCallback, useEffect, useState } from 'react'
import {
  Activity,
  AlertTriangle,
  BellRing,
  Minimize2,
  PlugZap,
  Radio,
  RefreshCw,
  ShieldCheck
} from 'lucide-react'
import { SettingsPanel } from '@/components/SettingsPanel'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle
} from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import {
  describeRelayState,
  describeTokenSource,
  formatContactTime,
  relayBadgeClassName,
  relayDotClassName
} from '@/lib/status'
import {
  ClearSecret,
  GetRelayStatus,
  GetSecretsStatus,
  GetSettings,
  GetShellStatus,
  HideToTray,
  ReconnectRelay,
  SaveSecret,
  UpdateSetting
} from '../wailsjs/go/main/App'
import { main } from '../wailsjs/go/models'
import type { secrets, store } from '../wailsjs/go/models'
import { EventsOn } from '../wailsjs/runtime/runtime'

const FALLBACK_STATUS: main.ShellStatus = {
  appName: 'Banshee',
  version: '0.1.0-dev',
  goVersion: 'preview',
  platform: 'windows/amd64',
  relayUrl: 'https://relay.example.com',
  relayState: 'standby',
  dbReady: false
}

const FALLBACK_RELAY_STATUS: main.RelayStatusSnapshot = main.RelayStatusSnapshot.createFrom({
  state: 'standby',
  relayUrl: 'https://relay.example.com',
  lastContactAt: undefined,
  lastReplayCount: undefined,
  error: undefined,
  maxSeq: 0,
  alertCount: 0,
  missedCount: 0,
  newestAlert: undefined,
  tokenSource: 'none'
})

const FALLBACK_SETTINGS: store.SettingsSnapshot = {
  relayUrl: 'https://relay.example.com',
  soundVolume: 0.75,
  soundMuted: false,
  eventTypeFilter: 'all',
  autoStart: false
}

const FALLBACK_SECRETS: secrets.SecretStatus = {
  encryptionAvailable: false,
  turingTokenSet: false,
  deviceTokenSet: false
}

export function App() {
  const [status, setStatus] = useState<main.ShellStatus>(FALLBACK_STATUS)
  const [relayStatus, setRelayStatus] = useState<main.RelayStatusSnapshot>(FALLBACK_RELAY_STATUS)
  const [settings, setSettings] = useState<store.SettingsSnapshot>(FALLBACK_SETTINGS)
  const [secretsStatus, setSecretsStatus] = useState<secrets.SecretStatus>(FALLBACK_SECRETS)
  const [loading, setLoading] = useState(true)
  const [reconnecting, setReconnecting] = useState(false)
  const [alarmArmed, setAlarmArmed] = useState(true)

  const refreshAll = useCallback(async () => {
    setLoading(true)
    try {
      const [nextStatus, nextRelay, nextSettings, nextSecrets] = await Promise.all([
        GetShellStatus(),
        GetRelayStatus(),
        GetSettings(),
        GetSecretsStatus()
      ])
      setStatus(nextStatus)
      setRelayStatus(nextRelay)
      setSettings(nextSettings)
      setSecretsStatus(nextSecrets)
    } catch {
      // Browser-only preview outside Wails uses fallback telemetry.
      setStatus(FALLBACK_STATUS)
      setRelayStatus(FALLBACK_RELAY_STATUS)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void refreshAll()
  }, [refreshAll])

  useEffect(() => {
    if (typeof window === 'undefined' || !('runtime' in window)) {
      return
    }
    const off = EventsOn('relay:status', (snap: main.RelayStatusSnapshot) => {
      if (snap && typeof snap === 'object') {
        setRelayStatus(snap)
        setStatus((prev) => ({
          ...prev,
          relayUrl: snap.relayUrl || prev.relayUrl,
          relayState: snap.state || prev.relayState
        }))
      }
    })
    return () => {
      if (typeof off === 'function') {
        off()
      }
    }
  }, [])

  const handleUpdateSetting = async (key: string, value: unknown) => {
    const updated = await UpdateSetting(key, value)
    setSettings(updated)
    const [nextStatus, nextRelay] = await Promise.all([GetShellStatus(), GetRelayStatus()])
    setStatus(nextStatus)
    setRelayStatus(nextRelay)
  }

  const handleSaveSecret = async (name: string, plaintext: string) => {
    const updated = await SaveSecret(name, plaintext)
    setSecretsStatus(updated)
    const [nextStatus, nextRelay] = await Promise.all([GetShellStatus(), GetRelayStatus()])
    setStatus(nextStatus)
    setRelayStatus(nextRelay)
  }

  const handleClearSecret = async (name: string) => {
    const updated = await ClearSecret(name)
    setSecretsStatus(updated)
    const [nextStatus, nextRelay] = await Promise.all([GetShellStatus(), GetRelayStatus()])
    setStatus(nextStatus)
    setRelayStatus(nextRelay)
  }

  const handleReconnect = async () => {
    setReconnecting(true)
    try {
      const nextRelay = await ReconnectRelay()
      setRelayStatus(nextRelay)
      setStatus((prev) => ({ ...prev, relayState: nextRelay.state, relayUrl: nextRelay.relayUrl }))
    } catch {
      // Ignore outside Wails runtime.
    } finally {
      setReconnecting(false)
    }
  }

  const relayBadge = describeRelayState(relayStatus.state)

  return (
    <div className="min-h-screen bg-background text-foreground">
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 px-6 py-8">
        <header className="flex flex-wrap items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="flex size-10 items-center justify-center rounded-lg border border-border bg-card text-destructive">
              <BellRing />
            </div>
            <div className="flex flex-col gap-0.5">
              <div className="flex items-center gap-2">
                <h1 className="font-heading text-xl font-semibold tracking-tight">
                  {status.appName}
                </h1>
                <Badge variant="outline" className="font-mono">
                  {status.version}
                </Badge>
                <Badge variant={status.dbReady ? 'secondary' : 'outline'}>
                  {status.dbReady ? 'SQLite Ready' : 'Preview Mode'}
                </Badge>
              </div>
              <p className="text-sm text-muted-foreground">
                Go + Wails alerting desktop shell for the Turing relay
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <Button
              variant={alarmArmed ? 'destructive' : 'outline'}
              size="sm"
              onClick={() => setAlarmArmed((prev) => !prev)}
            >
              <ShieldCheck data-icon="inline-start" />
              {alarmArmed ? 'Siren Armed' : 'Siren Muted'}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => void HideToTray()}
            >
              <Minimize2 data-icon="inline-start" />
              Hide to Tray
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={loading}
              onClick={() => void refreshAll()}
            >
              <RefreshCw data-icon="inline-start" />
              Refresh
            </Button>
          </div>
        </header>

        <Separator />

        <div className="grid gap-4 md:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Radio />
                Relay Link
              </CardTitle>
              <CardDescription>
                Live outbound WebSocket &amp; cursor replay telemetry
              </CardDescription>
              <CardAction>
                <Badge
                  variant={relayBadge.variant}
                  className={relayBadgeClassName(relayStatus.state)}
                >
                  <span
                    aria-hidden="true"
                    className={`size-1.5 rounded-full ${relayDotClassName(relayStatus.state)}`}
                  />
                  {relayBadge.label}
                </Badge>
              </CardAction>
            </CardHeader>
            <CardContent className="flex flex-col gap-3 text-sm">
              <div className="flex flex-col gap-1">
                <span className="text-xs text-muted-foreground">Relay Endpoint</span>
                <code className="truncate rounded-md border border-border bg-muted px-2.5 py-1.5 font-mono text-xs text-foreground">
                  {relayStatus.relayUrl}
                </code>
              </div>

              <div className="grid grid-cols-2 gap-3 text-xs">
                <div className="flex flex-col gap-0.5">
                  <span className="text-muted-foreground">Cursor (MAX seq)</span>
                  <span className="font-mono text-sm font-medium text-foreground">
                    #{relayStatus.maxSeq}
                  </span>
                </div>
                <div className="flex flex-col gap-0.5">
                  <span className="text-muted-foreground">Last Contact</span>
                  <span className="font-mono text-sm font-medium text-foreground">
                    {formatContactTime(relayStatus.lastContactAt)}
                  </span>
                </div>
                <div className="flex flex-col gap-0.5">
                  <span className="text-muted-foreground">Stored Alerts</span>
                  <span className="font-mono text-sm font-medium text-foreground">
                    {relayStatus.alertCount}
                    {relayStatus.missedCount > 0 ? ` (${relayStatus.missedCount} replayed)` : ''}
                  </span>
                </div>
                <div className="flex flex-col gap-0.5">
                  <span className="text-muted-foreground">Last Replay Burst</span>
                  <span className="font-mono text-sm font-medium text-foreground">
                    {relayStatus.lastReplayCount ?? '—'}
                  </span>
                </div>
              </div>

              {relayStatus.error && (
                <Alert variant="destructive">
                  <AlertTriangle />
                  <AlertTitle>Relay Diagnostic</AlertTitle>
                  <AlertDescription>{relayStatus.error}</AlertDescription>
                </Alert>
              )}
            </CardContent>
            <CardFooter className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
              <span>Auth: {describeTokenSource(relayStatus.tokenSource)}</span>
              <Button
                variant="outline"
                size="sm"
                disabled={reconnecting}
                onClick={() => void handleReconnect()}
              >
                <PlugZap data-icon="inline-start" />
                {reconnecting ? 'Dialing…' : 'Reconnect'}
              </Button>
            </CardFooter>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Activity />
                Native Runtime &amp; Feed
              </CardTitle>
              <CardDescription>
                Compiled Go backend bound to WebView2
              </CardDescription>
              <CardAction>
                <Badge variant="secondary" className="font-mono">
                  {status.platform}
                </Badge>
              </CardAction>
            </CardHeader>
            <CardContent className="flex flex-col gap-2 text-sm">
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">Go Toolchain</span>
                <span className="font-mono text-xs">{status.goVersion}</span>
              </div>
              <Separator />
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">Tray &amp; Lock</span>
                <span className="font-mono text-xs">Hide on close · Single instance</span>
              </div>
              <Separator />
              <div className="flex items-center justify-between gap-3">
                <span className="text-muted-foreground">Latest Stored Alert</span>
                <span className="truncate font-mono text-xs text-foreground">
                  {relayStatus.newestAlert
                    ? `${relayStatus.newestAlert.cameraName} · ${relayStatus.newestAlert.eventType}`
                    : 'No events received yet'}
                </span>
              </div>
            </CardContent>
            <CardFooter className="text-xs text-muted-foreground">
              Zero inbound ports; acks commit only after durable SQLite persistence.
            </CardFooter>
          </Card>
        </div>

        <SettingsPanel
          key={settings.relayUrl}
          settings={settings}
          secretsStatus={secretsStatus}
          onUpdateSetting={handleUpdateSetting}
          onSaveSecret={handleSaveSecret}
          onClearSecret={handleClearSecret}
        />
      </div>
    </div>
  )
}

export default App
