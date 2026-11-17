package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"banshee/internal/env"
	"banshee/internal/lifecycle"
	"banshee/internal/relay"
	"banshee/internal/secrets"
	"banshee/internal/store"
)

const (
	defaultRelayURL = "https://relay.example.com"
	appVersion      = "0.1.0-dev"
)

// TrayStatusSink receives live relay status labels for the Windows system tray.
type TrayStatusSink interface {
	SetRelayStatus(label string)
}

// ShellStatus describes the baseline desktop shell state exposed to the React UI.
type ShellStatus struct {
	AppName    string `json:"appName"`
	Version    string `json:"version"`
	GoVersion  string `json:"goVersion"`
	Platform   string `json:"platform"`
	RelayURL   string `json:"relayUrl"`
	RelayState string `json:"relayState"`
	DBReady    bool   `json:"dbReady"`
}

// RelayStatusSnapshot exposes live WebSocket health and alert cursor telemetry without secret values.
type RelayStatusSnapshot struct {
	State           string              `json:"state"`
	RelayURL        string              `json:"relayUrl"`
	LastContactAt   string              `json:"lastContactAt,omitempty"`
	LastReplayCount *int64              `json:"lastReplayCount"`
	Error           string              `json:"error,omitempty"`
	MaxSeq          int64               `json:"maxSeq"`
	AlertCount      int64               `json:"alertCount"`
	MissedCount     int64               `json:"missedCount"`
	NewestAlert     *store.AlertPreview `json:"newestAlert"`
	TokenSource     string              `json:"tokenSource"`
}

// App struct holds application context, lifecycle policy, relay client, and stores exposed to Wails.
type App struct {
	ctx       context.Context
	db        *sql.DB
	settings  *store.SettingsStore
	alerts    *store.AlertsStore
	secrets   *secrets.Store
	lifecycle *lifecycle.Policy
	initErr   error

	relayMu     sync.Mutex
	relayClient *relay.Client
	traySink    TrayStatusSink
	dialer      relay.Dialer
	probeAuth   relay.AuthProbeFunc
	wailsEvents bool
	onStatus    func(RelayStatusSnapshot)
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{
		lifecycle: lifecycle.NewPolicy(),
	}
}

// NewAppWithDeps builds an App backed by caller-provided dependencies (used in tests).
func NewAppWithDeps(db *sql.DB, cipher secrets.Cipher) *App {
	return &App{
		db:        db,
		settings:  store.NewSettingsStore(db),
		alerts:    store.NewAlertsStore(db),
		secrets:   secrets.NewStore(db, cipher),
		lifecycle: lifecycle.NewPolicy(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	_ = env.LoadDotEnvIfPresent(".env")

	dbPath, err := store.DefaultDBPath()
	if err != nil {
		a.initErr = err
		return
	}
	db, err := store.OpenDatabase(dbPath)
	if err != nil {
		a.initErr = err
		return
	}
	if _, err := store.ApplyMigrations(db, nil); err != nil {
		_ = db.Close()
		a.initErr = err
		return
	}

	a.db = db
	a.settings = store.NewSettingsStore(db)
	a.alerts = store.NewAlertsStore(db)
	a.secrets = secrets.NewStore(db, secrets.NewOSCipher())
	a.startRelay()
}

func (a *App) shutdown(_ context.Context) {
	a.stopRelay()
	if a.db != nil {
		_ = a.db.Close()
		a.db = nil
	}
}

func (a *App) setTraySink(sink TrayStatusSink) {
	a.relayMu.Lock()
	a.traySink = sink
	a.relayMu.Unlock()
	if sink != nil {
		sink.SetRelayStatus(formatTrayRelayLabel(a.GetRelayStatus().State))
	}
}

func (a *App) beforeClose(ctx context.Context) bool {
	if a.lifecycle != nil && a.lifecycle.ShouldPreventClose() {
		wailsRuntime.WindowHide(ctx)
		return true
	}
	return false
}

func (a *App) showWindow() {
	if a.ctx == nil {
		return
	}
	wailsRuntime.WindowShow(a.ctx)
	wailsRuntime.WindowUnminimise(a.ctx)
}

func (a *App) requestQuit() {
	if a.lifecycle != nil {
		a.lifecycle.RequestQuit()
	}
	if a.ctx != nil {
		wailsRuntime.Quit(a.ctx)
	}
}

func (a *App) ensureReady() error {
	if a.initErr != nil {
		return a.initErr
	}
	if a.settings == nil || a.secrets == nil || a.alerts == nil {
		return errors.New("desktop store is not initialized")
	}
	return nil
}

// Greet returns a greeting for the given name.
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Banshee armed for %s.", name)
}

// HideToTray hides the main window to the Windows notification area.
func (a *App) HideToTray() {
	if a.ctx != nil {
		wailsRuntime.WindowHide(a.ctx)
	}
}

// GetShellStatus returns current runtime metadata for the Banshee desktop shell.
func (a *App) GetShellStatus() ShellStatus {
	relaySnap := a.GetRelayStatus()
	return ShellStatus{
		AppName:    "Banshee",
		Version:    appVersion,
		GoVersion:  runtime.Version(),
		Platform:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		RelayURL:   relaySnap.RelayURL,
		RelayState: relaySnap.State,
		DBReady:    a.initErr == nil && a.db != nil,
	}
}

// GetRelayStatus returns live WebSocket relay health and SQLite alert cursor summary.
func (a *App) GetRelayStatus() RelayStatusSnapshot {
	a.relayMu.Lock()
	client := a.relayClient
	a.relayMu.Unlock()

	snap := RelayStatusSnapshot{
		State:       string(relay.StateStandby),
		RelayURL:    a.resolveRelayURL(),
		TokenSource: a.tokenSource(),
	}

	if client != nil {
		st := client.Status()
		snap.State = string(st.State)
		if st.RelayURL != "" {
			snap.RelayURL = st.RelayURL
		}
		if st.LastContactAt != nil {
			snap.LastContactAt = st.LastContactAt.UTC().Format(time.RFC3339)
		}
		snap.LastReplayCount = st.LastReplayCount
		snap.Error = st.Error
	}

	a.populateAlertTelemetry(&snap)
	return snap
}

// ReconnectRelay restarts the relay client immediately and returns the refreshed status snapshot.
func (a *App) ReconnectRelay() (RelayStatusSnapshot, error) {
	if err := a.ensureReady(); err != nil {
		return RelayStatusSnapshot{}, err
	}
	a.startRelay()
	return a.GetRelayStatus(), nil
}

// GetSettings returns all validated, non-secret settings.
func (a *App) GetSettings() (store.SettingsSnapshot, error) {
	if err := a.ensureReady(); err != nil {
		return store.SettingsSnapshot{}, err
	}
	return a.settings.GetAll()
}

// UpdateSetting validates and saves one allowlisted setting key, returning the updated snapshot.
func (a *App) UpdateSetting(key string, value any) (store.SettingsSnapshot, error) {
	if err := a.ensureReady(); err != nil {
		return store.SettingsSnapshot{}, err
	}
	if err := a.settings.Update(key, value); err != nil {
		return store.SettingsSnapshot{}, err
	}
	snap, err := a.settings.GetAll()
	if err != nil {
		return store.SettingsSnapshot{}, err
	}
	if key == "relay_url" {
		a.startRelay()
	}
	return snap, nil
}

// GetSecretsStatus reports token presence and DPAPI availability without returning secret values.
func (a *App) GetSecretsStatus() (secrets.SecretStatus, error) {
	if err := a.ensureReady(); err != nil {
		return secrets.SecretStatus{}, err
	}
	return a.secrets.Status()
}

// SaveSecret encrypts and persists an allowlisted secret using Windows DPAPI.
func (a *App) SaveSecret(name, plaintext string) (secrets.SecretStatus, error) {
	if err := a.ensureReady(); err != nil {
		return secrets.SecretStatus{}, err
	}
	if err := a.secrets.Save(name, plaintext); err != nil {
		return secrets.SecretStatus{}, err
	}
	st, err := a.secrets.Status()
	if err != nil {
		return secrets.SecretStatus{}, err
	}
	if name == "device_token" {
		a.startRelay()
	}
	return st, nil
}

// ClearSecret deletes a stored secret from SQLite and returns the updated status.
func (a *App) ClearSecret(name string) (secrets.SecretStatus, error) {
	if err := a.ensureReady(); err != nil {
		return secrets.SecretStatus{}, err
	}
	if err := a.secrets.Clear(name); err != nil {
		return secrets.SecretStatus{}, err
	}
	st, err := a.secrets.Status()
	if err != nil {
		return secrets.SecretStatus{}, err
	}
	if name == "device_token" {
		a.startRelay()
	}
	return st, nil
}

func (a *App) resolveRelayURL() string {
	relayURL := os.Getenv("BANSHEE_RELAY_URL")
	if relayURL == "" {
		relayURL = defaultRelayURL
	}
	if a.settings != nil {
		if snap, err := a.settings.GetAll(); err == nil && snap.RelayURL != "" {
			relayURL = snap.RelayURL
		}
	}
	return relayURL
}

func (a *App) tokenSource() string {
	if a.secrets != nil {
		if st, err := a.secrets.Status(); err == nil && st.DeviceTokenSet {
			return "dpapi"
		}
	}
	if tok := envDeviceToken(); tok != "" {
		return "env"
	}
	return "none"
}

func envDeviceToken() string {
	tok := strings.TrimSpace(os.Getenv("BANSHEE_DEVICE_TOKEN"))
	if tok == "" || tok == "replace-with-device-token" {
		return ""
	}
	return tok
}

func (a *App) resolveDeviceToken() (string, error) {
	if a.secrets != nil {
		st, err := a.secrets.Status()
		if err == nil && st.DeviceTokenSet {
			return a.secrets.Load("device_token")
		}
	}
	if tok := envDeviceToken(); tok != "" {
		return tok, nil
	}
	return "", nil
}

func (a *App) populateAlertTelemetry(snap *RelayStatusSnapshot) {
	if a.alerts == nil {
		return
	}
	if maxSeq, err := a.alerts.MaxSeq(); err == nil {
		snap.MaxSeq = maxSeq
	}
	if sum, err := a.alerts.Summary(); err == nil {
		snap.AlertCount = sum.Count
		snap.MissedCount = sum.Missed
		snap.NewestAlert = sum.Newest
	}
}

func (a *App) startRelay() {
	if a.alerts == nil {
		return
	}

	a.stopRelay()

	relayURL := a.resolveRelayURL()
	client := relay.NewClient(relay.ClientOptions{
		RelayURL:      relayURL,
		ClientVersion: appVersion,
		GetToken: func(_ context.Context) (string, error) {
			return a.resolveDeviceToken()
		},
		Store:     alertsStoreAdapter{store: a.alerts},
		Dial:      a.dialer,
		ProbeAuth: a.probeAuth,
	})

	client.OnStatus(func(st relay.Status) {
		a.relayMu.Lock()
		sink := a.traySink
		cb := a.onStatus
		wailsEvents := a.wailsEvents
		ctx := a.ctx
		a.relayMu.Unlock()

		if sink != nil {
			sink.SetRelayStatus(formatTrayRelayLabel(string(st.State)))
		}

		snap := RelayStatusSnapshot{
			State:           string(st.State),
			RelayURL:        st.RelayURL,
			LastReplayCount: st.LastReplayCount,
			Error:           st.Error,
			TokenSource:     a.tokenSource(),
		}
		if st.LastContactAt != nil {
			snap.LastContactAt = st.LastContactAt.UTC().Format(time.RFC3339)
		}
		a.populateAlertTelemetry(&snap)

		if cb != nil {
			cb(snap)
		}
		if wailsEvents && ctx != nil {
			wailsRuntime.EventsEmit(ctx, "relay:status", snap)
		}
	})

	parentCtx := a.ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	a.relayMu.Lock()
	a.relayClient = client
	a.relayMu.Unlock()

	client.Start(parentCtx)
}

func (a *App) stopRelay() {
	a.relayMu.Lock()
	client := a.relayClient
	a.relayClient = nil
	a.relayMu.Unlock()

	if client != nil {
		client.Stop()
	}
}

func formatTrayRelayLabel(state string) string {
	switch state {
	case string(relay.StateConnected):
		return "Connected"
	case string(relay.StateDegraded):
		return "Degraded"
	case string(relay.StateOffline):
		return "Offline"
	default:
		return "Standby"
	}
}

type alertsStoreAdapter struct {
	store *store.AlertsStore
}

func (a alertsStoreAdapter) MaxSeq() (int64, error) {
	return a.store.MaxSeq()
}

func (a alertsStoreAdapter) UpsertFromRelay(seq int64, ev relay.EventPayload, missed bool) error {
	_, err := a.store.UpsertFromRelay(seq, store.AlertUpsert{
		EventID:        ev.EventID,
		EventType:      ev.EventType,
		StagesReceived: ev.StagesReceived,
		CameraID:       *ev.Camera.ID,
		CameraName:     *ev.Camera.Name,
		SiteID:         *ev.Site.ID,
		SiteName:       *ev.Site.Name,
		OccurredAt:     *ev.OccurredAt,
		ReceivedAt:     *ev.ReceivedAt,
	}, missed)
	return err
}
