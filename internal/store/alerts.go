package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// UpsertResult describes the outcome of persisting a relay event by event_id and seq.
type UpsertResult string

const (
	UpsertInserted UpsertResult = "inserted"
	UpsertUpdated  UpsertResult = "updated"
	UpsertStale    UpsertResult = "stale"
)

// AlertUpsert holds the non-media fields persisted from a relay event frame.
// Media URLs and raw metadata are deliberately excluded per NFR-10 privacy rules.
type AlertUpsert struct {
	EventID        string
	EventType      string
	StagesReceived []string
	CameraID       int64
	CameraName     string
	SiteID         int64
	SiteName       string
	OccurredAt     string
	ReceivedAt     string
}

// AlertPreview summarizes the newest stored alert for dashboard telemetry.
type AlertPreview struct {
	CameraName string `json:"cameraName"`
	EventType  string `json:"eventType"`
	OccurredAt string `json:"occurredAt"`
}

// AlertsSummary reports aggregate stored alert counts for status telemetry.
type AlertsSummary struct {
	Count  int64         `json:"count"`
	Missed int64         `json:"missed"`
	Newest *AlertPreview `json:"newest"`
}

// AlertsStore manages idempotent alert persistence and cursor queries in SQLite.
type AlertsStore struct {
	mu sync.Mutex
	db *sql.DB
}

// NewAlertsStore wraps a migrated SQLite database for alert storage.
func NewAlertsStore(db *sql.DB) *AlertsStore {
	return &AlertsStore{db: db}
}

// MaxSeq returns the highest stored relay sequence cursor, or 0 when the alerts table is empty.
func (s *AlertsStore) MaxSeq() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var v sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(seq) FROM alerts`).Scan(&v); err != nil {
		return 0, fmt.Errorf("query max seq: %w", err)
	}
	if !v.Valid {
		return 0, nil
	}
	return v.Int64, nil
}

// UpsertFromRelay idempotently inserts or enriches an alert without ever overwriting local operator state or persisting media URLs.
func (s *AlertsStore) UpsertFromRelay(seq int64, ev AlertUpsert, missed bool) (UpsertResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stages := ev.StagesReceived
	if stages == nil {
		stages = []string{}
	}
	stagesJSON, err := json.Marshal(stages)
	if err != nil {
		return "", fmt.Errorf("marshal stages_received: %w", err)
	}

	var storedSeq int64
	err = s.db.QueryRow(`SELECT seq FROM alerts WHERE event_id = ?`, ev.EventID).Scan(&storedSeq)
	if errors.Is(err, sql.ErrNoRows) {
		missedInt := 0
		if missed {
			missedInt = 1
		}
		if _, err := s.db.Exec(
			`INSERT INTO alerts (
				event_id, seq, event_type, stages_received, camera_id, camera_name,
				site_id, site_name, occurred_at, received_at, status, missed
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'new', ?)`,
			ev.EventID,
			seq,
			ev.EventType,
			string(stagesJSON),
			ev.CameraID,
			ev.CameraName,
			ev.SiteID,
			ev.SiteName,
			ev.OccurredAt,
			ev.ReceivedAt,
			missedInt,
		); err != nil {
			return "", fmt.Errorf("insert alert %q: %w", ev.EventID, err)
		}
		return UpsertInserted, nil
	}
	if err != nil {
		return "", fmt.Errorf("lookup alert %q: %w", ev.EventID, err)
	}

	if seq <= storedSeq {
		return UpsertStale, nil
	}

	if _, err := s.db.Exec(
		`UPDATE alerts
		 SET seq = ?, stages_received = ?, event_type = ?, camera_id = ?, camera_name = ?,
		     site_id = ?, site_name = ?, occurred_at = ?, received_at = ?
		 WHERE event_id = ?`,
		seq,
		string(stagesJSON),
		ev.EventType,
		ev.CameraID,
		ev.CameraName,
		ev.SiteID,
		ev.SiteName,
		ev.OccurredAt,
		ev.ReceivedAt,
		ev.EventID,
	); err != nil {
		return "", fmt.Errorf("update alert %q: %w", ev.EventID, err)
	}
	return UpsertUpdated, nil
}

// Summary returns total alerts, missed replay count, and the newest alert preview.
func (s *AlertsStore) Summary() (AlertsSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var sum AlertsSummary
	if err := s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(missed), 0) FROM alerts`).Scan(&sum.Count, &sum.Missed); err != nil {
		return AlertsSummary{}, fmt.Errorf("query alerts summary: %w", err)
	}

	var preview AlertPreview
	err := s.db.QueryRow(
		`SELECT camera_name, event_type, occurred_at FROM alerts ORDER BY seq DESC LIMIT 1`,
	).Scan(&preview.CameraName, &preview.EventType, &preview.OccurredAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return AlertsSummary{}, fmt.Errorf("query newest alert: %w", err)
	}
	if err == nil {
		sum.Newest = &preview
	}
	return sum, nil
}
