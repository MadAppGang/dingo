package main

import (
	"fmt"
	"os"
)

func readConfig(path string) ([]byte, error) {
	// DINGO:GENERATED:START error_propagation
	__tmp0, __err0 := os.ReadFile(path)
	if __err0 != nil {
		return nil, fmt.Errorf("failed to read config: %w", __err0)
	}
	data := __tmp0
	// DINGO:GENERATED:END

	return data, nil
}

func fetchUser(id int) (*User, error) {
	// DINGO:GENERATED:START error_propagation
	__tmp1, __err1 := connect()
	if __err1 != nil {
		return nil, __err1
	}
	db := __tmp1
	// DINGO:GENERATED:END

	// DINGO:GENERATED:START error_propagation
	__tmp2, __err2 := db.query(id)
	if __err2 != nil {
		return nil, __err2
	}
	user := __tmp2
	// DINGO:GENERATED:END

	return user, nil
}

type User struct {
	ID   int
	Name string
}

type DB struct{}

func connect() (*DB, error) {
	return &DB{}, nil
}

func (db *DB) query(id int) (*User, error) {
	return &User{ID: id, Name: "Test"}, nil
}

func main() {
	data, err := readConfig("config.json")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Data:", string(data))
}
