package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

type ObsMsg struct {
	Type   string                 `json:"type"`
	Obs    []float64              `json:"obs"`
	Reward float64                `json:"reward"`
	Done   bool                   `json:"done"`
	Info   map[string]interface{} `json:"info,omitempty"`
}

type ClientMsg struct {
	Type   string `json:"type"`
	Action int    `json:"action,omitempty"`
}

func writeMsg(w io.Writer, msg ObsMsg) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

func readMsg(r io.Reader) (ClientMsg, error) {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return ClientMsg{}, err
		}
		return ClientMsg{}, io.EOF
	}
	var msg ClientMsg
	if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
		return ClientMsg{}, err
	}
	return msg, nil
}
