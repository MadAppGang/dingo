package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Host string
	Port int
}

func pipeline(path string) (*Config, error) {
	__tmp0, __err0 := os.ReadFile(path)

	if __err0 != nil {
		return nil, fmt.Errorf("failed to read config: %w", __err0)
	}
	// dingo:e:1
	var data = __tmp0
	var cfg Config
	__tmp1, __err1 := json.Unmarshal(data, &cfg)

	if __err1 != nil {
		return nil, fmt.Errorf("failed to parse config: %w", __err1)
	}
	// dingo:e:1
	var err = __tmp1
	return &cfg, nil
}
