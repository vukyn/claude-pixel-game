package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestEncodeObsMsg(t *testing.T) {
	msg := ObsMsg{
		Type:   "obs",
		Obs:    []float64{0.5, 0.3},
		Reward: 1.5,
		Done:   false,
		Info:   map[string]interface{}{"score": 10},
	}
	var buf bytes.Buffer
	if err := writeMsg(&buf, msg); err != nil {
		t.Fatalf("writeMsg: %v", err)
	}
	line := buf.String()
	if line[len(line)-1] != '\n' {
		t.Error("message should end with newline")
	}
	var decoded ObsMsg
	if err := json.Unmarshal([]byte(line), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Type != "obs" {
		t.Errorf("type = %q, want obs", decoded.Type)
	}
	if len(decoded.Obs) != 2 {
		t.Errorf("obs len = %d, want 2", len(decoded.Obs))
	}
}

func TestDecodeActionMsg(t *testing.T) {
	input := `{"type":"action","action":3}` + "\n"
	msg, err := readMsg(bytes.NewBufferString(input))
	if err != nil {
		t.Fatalf("readMsg: %v", err)
	}
	if msg.Type != "action" {
		t.Errorf("type = %q, want action", msg.Type)
	}
	if msg.Action != 3 {
		t.Errorf("action = %d, want 3", msg.Action)
	}
}

func TestDecodeResetMsg(t *testing.T) {
	input := `{"type":"reset"}` + "\n"
	msg, err := readMsg(bytes.NewBufferString(input))
	if err != nil {
		t.Fatalf("readMsg: %v", err)
	}
	if msg.Type != "reset" {
		t.Errorf("type = %q, want reset", msg.Type)
	}
}
