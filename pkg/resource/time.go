package resource

import (
	"fmt"
)

func FormatPlayTime(seconds int64) string {
	if seconds == 0 {
		return "0m"
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
