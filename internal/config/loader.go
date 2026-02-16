package config

import (
	"encoding/json"
	"os"
)

type GlobalSettings struct {
	AspectRatio    string `json:"aspect_ratio"`
	NegativePrompt string `json:"negative_prompt"`
}

type Segment struct {
	Duration int    `json:"duration"`
	Prompt   string `json:"prompt"`
}

type Config struct {
	GlobalSettings GlobalSettings `json:"global_settings"`
	Segments       []Segment      `json:"segments"`
}

func Load(filepath string) (*Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
