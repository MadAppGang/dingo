package main

import (
	"fmt"
	"os"
)

func readUserConfig(username string) ([]byte, error) {
	path := "/home/" + username + "/config.json"
	__tmp0, __err0 := os.ReadFile(path)

	if __err0 != nil {
		return nil, fmt.Errorf("failed to read user config: %w", __err0)
	}
	// dingo:e:1
	var data = __tmp0
	return data, nil
}
