package relay

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Server frame types defined by the frozen turing-api relay protocol.
const (
	FrameTypeEvent      = "event"
	FrameTypeReplayDone = "replay_done"
	FrameTypePing       = "ping"
	FrameTypePong       = "pong"
)

// MediumFile represents one media file descriptor in a relay event frame.
type MediumFile struct {
	URL string `json:"url"`
}

// MediumFrame represents a named media collection (e.g. snapshot, video).
type MediumFrame struct {
	Name  string       `json:"name"`
	Files []MediumFile `json:"files"`
}

// EntityRef identifies a camera or site in a relay event frame.
type EntityRef struct {
	ID   *int64  `json:"id"`
	Name *string `json:"name"`
}

// EventPayload is the validated relay event payload.
type EventPayload struct {
	EventID        string            `json:"event_id"`
	EventType      string            `json:"event_type"`
	StagesReceived []string          `json:"stages_received"`
	Stage          *string           `json:"stage"`
	Camera         *EntityRef        `json:"camera"`
	Site           *EntityRef        `json:"site"`
	OccurredAt     *string           `json:"occurred_at"`
	ReceivedAt     *string           `json:"received_at"`
	Mediums        []MediumFrame     `json:"mediums"`
	Metadata       []json.RawMessage `json:"metadata"`
}

// ServerFrame represents one validated JSON text frame received from the relay.
type ServerFrame struct {
	Type    string        `json:"type"`
	Seq     *int64        `json:"seq,omitempty"`
	Event   *EventPayload `json:"event,omitempty"`
	Count   *int64        `json:"count,omitempty"`
	FromSeq *int64        `json:"from_seq,omitempty"`
	ToSeq   *int64        `json:"to_seq,omitempty"`
	T       *float64      `json:"t,omitempty"`
}

type helloWire struct {
	Type          string `json:"type"`
	Cursor        int64  `json:"cursor"`
	ClientVersion string `json:"client_version"`
}

type ackWire struct {
	Type string `json:"type"`
	Seq  int64  `json:"seq"`
}

type pongWire struct {
	Type string   `json:"type"`
	T    *float64 `json:"t,omitempty"`
}

// EncodeHello builds the opening client hello frame with a non-negative seq cursor.
func EncodeHello(cursor int64, clientVersion string) ([]byte, error) {
	if cursor < 0 {
		return nil, errors.New("cursor must be a non-negative integer")
	}
	return json.Marshal(helloWire{
		Type:          "hello",
		Cursor:        cursor,
		ClientVersion: clientVersion,
	})
}

// EncodeAck builds a cumulative acknowledgement frame for a durably stored seq.
func EncodeAck(seq int64) ([]byte, error) {
	if seq < 0 {
		return nil, errors.New("seq must be a non-negative integer")
	}
	return json.Marshal(ackWire{
		Type: "ack",
		Seq:  seq,
	})
}

// EncodePong builds a liveness reply echoing the ping timestamp when present.
func EncodePong(t *float64) ([]byte, error) {
	return json.Marshal(pongWire{
		Type: "pong",
		T:    t,
	})
}

// ParseServerFrame unmarshals and validates an inbound relay JSON frame without leaking raw content in errors.
func ParseServerFrame(raw []byte) (*ServerFrame, error) {
	var frame ServerFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errors.New("malformed relay JSON frame")
	}

	switch frame.Type {
	case FrameTypeEvent:
		if err := validateEventFrame(&frame); err != nil {
			return nil, err
		}
		return &frame, nil
	case FrameTypeReplayDone:
		if frame.Count == nil || *frame.Count < 0 {
			return nil, errors.New("replay_done frame requires non-negative count")
		}
		if frame.FromSeq == nil || *frame.FromSeq < 0 {
			return nil, errors.New("replay_done frame requires non-negative from_seq")
		}
		if frame.ToSeq == nil || *frame.ToSeq < 0 {
			return nil, errors.New("replay_done frame requires non-negative to_seq")
		}
		return &frame, nil
	case FrameTypePing, FrameTypePong:
		return &frame, nil
	default:
		return nil, fmt.Errorf("unsupported relay frame type %q", frame.Type)
	}
}

func validateEventFrame(frame *ServerFrame) error {
	if frame.Seq == nil || *frame.Seq < 0 {
		return errors.New("event frame requires non-negative seq")
	}
	ev := frame.Event
	if ev == nil {
		return errors.New("event frame requires event payload")
	}
	if ev.EventID == "" {
		return errors.New("event payload requires non-empty event_id")
	}
	if ev.EventType != "people" && ev.EventType != "vehicle" {
		return errors.New("event payload requires event_type people or vehicle")
	}
	if ev.StagesReceived == nil {
		return errors.New("event payload requires stages_received array")
	}
	if ev.Camera == nil || ev.Camera.ID == nil || ev.Camera.Name == nil {
		return errors.New("event payload requires camera id and name")
	}
	if ev.Site == nil || ev.Site.ID == nil || ev.Site.Name == nil {
		return errors.New("event payload requires site id and name")
	}
	if ev.OccurredAt == nil || ev.ReceivedAt == nil {
		return errors.New("event payload requires occurred_at and received_at")
	}
	if ev.Mediums == nil {
		return errors.New("event payload requires mediums array")
	}
	return nil
}
