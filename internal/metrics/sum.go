package metrics

// SumInt64 sums a slice of int64 values (used for lightweight aggregation in tests and tooling).
func SumInt64(vals []int64) int64 {
	var s int64
	for _, v := range vals {
		s += v
	}
	return s
}
