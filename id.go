package main

import (
	"strconv"
	"time"
)

func generateUniqueID(items []handoffRecord, now time.Time) string {
	base := now.UTC().Format("20060102-150405")
	if !idExists(items, base) {
		return base
	}

	for i := 2; i <= 999; i++ {
		candidate := base + "-" + formatIDSuffix(i)
		if !idExists(items, candidate) {
			return candidate
		}
	}
	return ""
}

func idExists(items []handoffRecord, id string) bool {
	for _, it := range items {
		if it.ID == id {
			return true
		}
	}
	return false
}

func formatIDSuffix(n int) string {
	if n < 10 {
		return "00" + strconv.Itoa(n)
	}
	if n < 100 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
