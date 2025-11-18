package main

import (
	"os"
	"strconv"
)

func processData(path string) (int, error) {
	__tmp0, __err0 := os.ReadFile(path)

	if __err0 != nil {
		return 0, __err0
	}
	// dingo:e:1
	var data = __tmp0
	__tmp1, __err1 := strconv.Atoi(string(data))

	if __err1 != nil {
		return 0, __err1
	}

	return __tmp1, nil
}
