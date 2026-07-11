package inmath

import "testing"

func TestCircle_WrapsBelowMin(t *testing.T) {
	tests := []struct {
		val, min, max, want int
	}{
		{-1, 0, 5, 5},
		{-10, 0, 5, 5},
		{-1, 1, 3, 3},
	}
	for _, tt := range tests {
		got := Circle(tt.val, tt.min, tt.max)
		if got != tt.want {
			t.Errorf("Circle(%d, %d, %d) = %d, want %d", tt.val, tt.min, tt.max, got, tt.want)
		}
	}
}

func TestCircle_WrapsAboveMax(t *testing.T) {
	tests := []struct {
		val, min, max, want int
	}{
		{6, 0, 5, 0},
		{100, 0, 5, 0},
		{4, 1, 3, 1},
	}
	for _, tt := range tests {
		got := Circle(tt.val, tt.min, tt.max)
		if got != tt.want {
			t.Errorf("Circle(%d, %d, %d) = %d, want %d", tt.val, tt.min, tt.max, got, tt.want)
		}
	}
}

func TestCircle_WithinRange(t *testing.T) {
	tests := []struct {
		val, min, max, want int
	}{
		{3, 0, 5, 3},
		{0, 0, 5, 0},
		{5, 0, 5, 5},
		{2, 1, 3, 2},
		{1, 1, 3, 1},
		{3, 1, 3, 3},
	}
	for _, tt := range tests {
		got := Circle(tt.val, tt.min, tt.max)
		if got != tt.want {
			t.Errorf("Circle(%d, %d, %d) = %d, want %d", tt.val, tt.min, tt.max, got, tt.want)
		}
	}
}

func TestCircle_SingleValue(t *testing.T) {
	tests := []struct {
		val, min, max, want int
	}{
		{0, 0, 0, 0},
		{-1, 0, 0, 0},
		{1, 0, 0, 0},
	}
	for _, tt := range tests {
		got := Circle(tt.val, tt.min, tt.max)
		if got != tt.want {
			t.Errorf("Circle(%d, %d, %d) = %d, want %d", tt.val, tt.min, tt.max, got, tt.want)
		}
	}
}

func TestCircle_NegativeRange(t *testing.T) {
	tests := []struct {
		val, min, max, want int
	}{
		{-3, -5, -1, -3},
		{-6, -5, -1, -1},
		{0, -5, -1, -5},
		{-5, -5, -1, -5},
		{-1, -5, -1, -1},
	}
	for _, tt := range tests {
		got := Circle(tt.val, tt.min, tt.max)
		if got != tt.want {
			t.Errorf("Circle(%d, %d, %d) = %d, want %d", tt.val, tt.min, tt.max, got, tt.want)
		}
	}
}
