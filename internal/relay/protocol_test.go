package relay

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProtocolEncodeClientFrames(t *testing.T) {
	hello, err := EncodeHello(41, "0.1.0-dev")
	if err != nil {
		t.Fatalf("EncodeHello failed: %v", err)
	}
	if string(hello) != `{"type":"hello","cursor":41,"client_version":"0.1.0-dev"}` {
		t.Fatalf("unexpected hello frame: %s", hello)
	}

	if _, err := EncodeHello(-1, "0.1.0"); err == nil {
		t.Fatal("expected error for negative cursor")
	}

	ack, err := EncodeAck(42)
	if err != nil {
		t.Fatalf("EncodeAck failed: %v", err)
	}
	if string(ack) != `{"type":"ack","seq":42}` {
		t.Fatalf("unexpected ack frame: %s", ack)
	}

	if _, err := EncodeAck(-5); err == nil {
		t.Fatal("expected error for negative seq")
	}

	ts := float64(1789257731)
	pongWithT, err := EncodePong(&ts)
	if err != nil {
		t.Fatalf("EncodePong with t failed: %v", err)
	}
	if string(pongWithT) != `{"type":"pong","t":1789257731}` {
		t.Fatalf("unexpected pong frame: %s", pongWithT)
	}

	pongBare, err := EncodePong(nil)
	if err != nil {
		t.Fatalf("EncodePong without t failed: %v", err)
	}
	if string(pongBare) != `{"type":"pong"}` {
		t.Fatalf("unexpected bare pong frame: %s", pongBare)
	}
}

func TestProtocolParseAndValidateServerFrames(t *testing.T) {
	validEvent := `{
		"type":"event",
		"seq":41828,
		"event":{
			"event_id":"evt-123",
			"event_type":"vehicle",
			"stages_received":["created","processed"],
			"stage":"processed",
			"camera":{"id":10577,"name":"IP Camera 06"},
			"site":{"id":11120,"name":"Demo Site 1"},
			"occurred_at":"2026-11-13T15:00:00Z",
			"received_at":"2026-11-13T15:00:01Z",
			"mediums":[{"name":"snapshot","files":[{"url":"https://example.com/snap.jpg"}]}],
			"metadata":[{"color":"black"}]
		}
	}`

	frame, err := ParseServerFrame([]byte(validEvent))
	if err != nil {
		t.Fatalf("expected valid event frame to parse, got error: %v", err)
	}
	if frame.Type != FrameTypeEvent || frame.Seq == nil || *frame.Seq != 41828 {
		t.Fatalf("unexpected parsed frame metadata: %+v", frame)
	}
	if frame.Event == nil || frame.Event.EventID != "evt-123" || frame.Event.EventType != "vehicle" {
		t.Fatalf("unexpected event payload: %+v", frame.Event)
	}

	replayDone, err := ParseServerFrame([]byte(`{"type":"replay_done","count":0,"from_seq":0,"to_seq":0}`))
	if err != nil {
		t.Fatalf("expected valid replay_done frame to parse, got error: %v", err)
	}
	if replayDone.Type != FrameTypeReplayDone || replayDone.Count == nil || *replayDone.Count != 0 {
		t.Fatalf("unexpected replay_done frame: %+v", replayDone)
	}

	ping, err := ParseServerFrame([]byte(`{"type":"ping","t":1789257731}`))
	if err != nil {
		t.Fatalf("expected valid ping frame to parse, got error: %v", err)
	}
	if ping.Type != FrameTypePing || ping.T == nil || *ping.T != 1789257731 {
		t.Fatalf("unexpected ping frame: %+v", ping)
	}
}

func TestProtocolRejectsMalformedOrInvalidFrames(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{name: "invalid json", raw: `{not json`},
		{name: "unknown type", raw: `{"type":"unknown"}`},
		{name: "event missing seq", raw: `{"type":"event","event":{"event_id":"e1","event_type":"people","stages_received":[],"camera":{"id":1,"name":"Cam"},"site":{"id":1,"name":"Site"},"occurred_at":"2026-01-01T00:00:00Z","received_at":"2026-01-01T00:00:01Z","mediums":[]}}`},
		{name: "event negative seq", raw: `{"type":"event","seq":-1,"event":{"event_id":"e1","event_type":"people","stages_received":[],"camera":{"id":1,"name":"Cam"},"site":{"id":1,"name":"Site"},"occurred_at":"2026-01-01T00:00:00Z","received_at":"2026-01-01T00:00:01Z","mediums":[]}}`},
		{name: "event invalid event_type", raw: `{"type":"event","seq":1,"event":{"event_id":"e1","event_type":"animal","stages_received":[],"camera":{"id":1,"name":"Cam"},"site":{"id":1,"name":"Site"},"occurred_at":"2026-01-01T00:00:00Z","received_at":"2026-01-01T00:00:01Z","mediums":[]}}`},
		{name: "event empty event_id", raw: `{"type":"event","seq":1,"event":{"event_id":"","event_type":"people","stages_received":[],"camera":{"id":1,"name":"Cam"},"site":{"id":1,"name":"Site"},"occurred_at":"2026-01-01T00:00:00Z","received_at":"2026-01-01T00:00:01Z","mediums":[]}}`},
		{name: "replay_done negative count", raw: `{"type":"replay_done","count":-1,"from_seq":0,"to_seq":0}`},
		{name: "replay_done missing to_seq", raw: `{"type":"replay_done","count":1,"from_seq":0}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseServerFrame([]byte(tc.raw)); err == nil {
				t.Fatalf("expected frame %q to be rejected", tc.raw)
			}
		})
	}

	// Ensure marshaled error strings never echo sensitive payloads blindly.
	_, err := ParseServerFrame([]byte(`{"type":"event","seq":-1,"secret":"tok-123"}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "tok-123") {
		t.Fatalf("parse error leaked raw frame content: %s", err.Error())
	}
	_ = json.Valid
}
