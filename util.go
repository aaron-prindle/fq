package fq

func max(a, b uint64) uint64 {
	if a >= b {
		return a
	}
	return b
}

func min(a, b uint64) uint64 {
	if a <= b {
		return a
	}
	return b
}
