package db

import "time"

func Now() string {
	now := time.Now().Format(time.RFC3339)
	return now
}
