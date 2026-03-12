package handoffid

import (
	"strconv"
	"time"
)

func Generate(existing []string, now time.Time) string {
	base := now.UTC().Format("20060102-150405")
	if !exists(existing, base) {
		return base
	}

	for i := 2; i <= 999; i++ {
		candidate := base + "-" + formatSuffix(i)
		if !exists(existing, candidate) {
			return candidate
		}
	}
	return ""
}

func exists(existing []string, id string) bool {
	for _, item := range existing {
		if item == id {
			return true
		}
	}
	return false
}

func formatSuffix(n int) string {
	if n < 10 {
		return "00" + strconv.Itoa(n)
	}
	if n < 100 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
