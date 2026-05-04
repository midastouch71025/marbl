package consumer

import "time"

// StaleCutoff returns the last_update_time threshold below which processing rows are stale.
func StaleCutoff(now time.Time, staleSeconds float64) float64 {
	return float64(now.UnixNano())/1e9 - staleSeconds
}
