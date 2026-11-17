package store

import (
	"testing"
)

func openTestAlertsStore(t *testing.T) *AlertsStore {
	t.Helper()
	db, err := OpenDatabase(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := ApplyMigrations(db, nil); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return NewAlertsStore(db)
}

func TestAlertsStoreIdempotencyPreservesLocalStateAndPrivacy(t *testing.T) {
	alerts := openTestAlertsStore(t)

	maxSeq, err := alerts.MaxSeq()
	if err != nil {
		t.Fatalf("MaxSeq on empty table failed: %v", err)
	}
	if maxSeq != 0 {
		t.Fatalf("expected MaxSeq 0 on empty table, got %d", maxSeq)
	}

	evCreated := AlertUpsert{
		EventID:        "evt-100",
		EventType:      "vehicle",
		StagesReceived: []string{"created"},
		CameraID:       10577,
		CameraName:     "Gate Cam",
		SiteID:         11120,
		SiteName:       "Yard",
		OccurredAt:     "2026-11-13T15:00:00Z",
		ReceivedAt:     "2026-11-13T15:00:01Z",
	}

	res, err := alerts.UpsertFromRelay(42, evCreated, true)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if res != UpsertInserted {
		t.Fatalf("expected inserted, got %s", res)
	}

	// Simulate operator acknowledging and setting sound_played locally.
	if _, err := alerts.db.Exec(
		`UPDATE alerts SET status = 'acknowledged', acknowledged_at = '2026-11-13T15:01:00Z', sound_played = 1 WHERE event_id = ?`,
		"evt-100",
	); err != nil {
		t.Fatalf("simulate local acknowledge failed: %v", err)
	}

	// Stale/duplicate seq <= 42 must be ignored.
	res, err = alerts.UpsertFromRelay(42, evCreated, false)
	if err != nil {
		t.Fatalf("stale upsert failed: %v", err)
	}
	if res != UpsertStale {
		t.Fatalf("expected stale on equal seq, got %s", res)
	}

	// Merged processed update at seq 45 updates relay fields without resetting local status/missed/sound_played.
	evProcessed := evCreated
	evProcessed.StagesReceived = []string{"created", "processed"}
	evProcessed.CameraName = "Gate Cam Renamed"
	res, err = alerts.UpsertFromRelay(45, evProcessed, false)
	if err != nil {
		t.Fatalf("update upsert failed: %v", err)
	}
	if res != UpsertUpdated {
		t.Fatalf("expected updated on higher seq, got %s", res)
	}

	var status, ackAt, camName, stages string
	var snapshotURL, clipURL, metaSummary *string
	var soundPlayed, missed int
	err = alerts.db.QueryRow(
		`SELECT status, acknowledged_at, camera_name, stages_received, snapshot_url, clip_url, metadata_summary, sound_played, missed FROM alerts WHERE event_id = ?`,
		"evt-100",
	).Scan(&status, &ackAt, &camName, &stages, &snapshotURL, &clipURL, &metaSummary, &soundPlayed, &missed)
	if err != nil {
		t.Fatalf("query alert row failed: %v", err)
	}
	if status != "acknowledged" || ackAt != "2026-11-13T15:01:00Z" {
		t.Fatalf("local acknowledgement was overwritten: status=%q ackAt=%q", status, ackAt)
	}
	if soundPlayed != 1 || missed != 1 {
		t.Fatalf("local flags overwritten: sound_played=%d missed=%d", soundPlayed, missed)
	}
	if camName != "Gate Cam Renamed" || stages != `["created","processed"]` {
		t.Fatalf("relay merge did not update fields: cam=%q stages=%q", camName, stages)
	}
	if snapshotURL != nil || clipURL != nil || metaSummary != nil {
		t.Fatalf("privacy violation: media/metadata columns must stay NULL")
	}

	maxSeq, err = alerts.MaxSeq()
	if err != nil {
		t.Fatalf("MaxSeq failed: %v", err)
	}
	if maxSeq != 45 {
		t.Fatalf("expected MaxSeq 45, got %d", maxSeq)
	}

	sum, err := alerts.Summary()
	if err != nil {
		t.Fatalf("Summary failed: %v", err)
	}
	if sum.Count != 1 || sum.Missed != 1 || sum.Newest == nil || sum.Newest.CameraName != "Gate Cam Renamed" {
		t.Fatalf("unexpected summary: %+v", sum)
	}
}
