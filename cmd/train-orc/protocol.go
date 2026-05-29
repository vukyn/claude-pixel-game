package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

type OrcObsMsg struct {
	Type         string         `json:"type"`
	PlayerObs    []float64      `json:"player_obs"`
	PlayerReward float64        `json:"player_reward"`
	OrcObs       [][]float64    `json:"orc_obs"`
	OrcRewards   []float64      `json:"orc_rewards"`
	OrcDones     []bool         `json:"orc_dones"`
	Done         bool           `json:"done"`
	Info         map[string]any `json:"info,omitempty"`
}

type OrcClientMsg struct {
	Type         string `json:"type"`
	PlayerAction int    `json:"player_action,omitempty"`
	OrcActions   []int  `json:"orc_actions,omitempty"`
}

func writeOrcMsg(w io.Writer, msg OrcObsMsg) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

func readOrcMsg(r io.Reader) (OrcClientMsg, error) {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return OrcClientMsg{}, err
		}
		return OrcClientMsg{}, io.EOF
	}
	var msg OrcClientMsg
	if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
		return OrcClientMsg{}, err
	}
	return msg, nil
}
