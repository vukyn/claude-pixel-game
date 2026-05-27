package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestEncodeOrcObsMsg(t *testing.T) {
	msg := OrcObsMsg{
		Type:       "obs",
		PlayerObs:  []float64{0.5, 0.3},
		OrcObs:     [][]float64{{0.1, 0.2}, {0.3, 0.4}},
		OrcRewards: []float64{1.0, -0.5},
		OrcDones:   []bool{false, true},
		Done:       false,
	}
	var buf bytes.Buffer
	if err := writeOrcMsg(&buf, msg); err != nil {
		t.Fatalf("writeOrcMsg: %v", err)
	}
	line := buf.String()
	if line[len(line)-1] != '\n' {
		t.Error("should end with newline")
	}
	var decoded OrcObsMsg
	if err := json.Unmarshal([]byte(line), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.OrcObs) != 2 {
		t.Errorf("orc obs count = %d, want 2", len(decoded.OrcObs))
	}
}

func TestDecodeOrcActionMsg(t *testing.T) {
	input := `{"type":"action","player_action":2,"orc_actions":[1,3]}` + "\n"
	msg, err := readOrcMsg(bytes.NewBufferString(input))
	if err != nil {
		t.Fatalf("readOrcMsg: %v", err)
	}
	if msg.PlayerAction != 2 {
		t.Errorf("player_action = %d, want 2", msg.PlayerAction)
	}
	if len(msg.OrcActions) != 2 {
		t.Errorf("orc_actions len = %d, want 2", len(msg.OrcActions))
	}
}
