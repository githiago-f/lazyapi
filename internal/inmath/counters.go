// Package inmath implements little utils functions for math operations
package inmath

func Circle(val, min, max int) int {
	switch {
	case val < min:
		return max
	case val > max:
		return min
	default:
		return val
	}
}
