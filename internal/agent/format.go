package agent

import "strconv"

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func formatInt(v int64) string {
	return strconv.FormatInt(v, 10)
}
