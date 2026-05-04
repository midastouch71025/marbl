package producer

// ShouldPauseGeneration returns true when backlog is at or above the configured cap.
// A non-positive max disables backlog-based pausing.
func ShouldPauseGeneration(backlog, maxBacklog int64) bool {
	if maxBacklog <= 0 {
		return false
	}
	return backlog >= maxBacklog
}
