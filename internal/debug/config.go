package debug

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type Section struct {
	Title  string   `json:"title"`
	Fields []string `json:"fields"`
}

type Config struct {
	Sections []Section `json:"sections"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read debug config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse debug config: %w", err)
	}
	var unknown []string
	for _, s := range cfg.Sections {
		for _, f := range s.Fields {
			if _, ok := Catalog[f]; !ok {
				unknown = append(unknown, f)
			}
		}
	}
	if len(unknown) > 0 {
		valid := make([]string, 0, len(Catalog))
		for k := range Catalog {
			valid = append(valid, k)
		}
		sort.Strings(valid)
		return nil, fmt.Errorf("debug config references unknown fields %v; valid keys: %v", unknown, valid)
	}
	return &cfg, nil
}
