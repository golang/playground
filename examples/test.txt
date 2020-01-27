package main

import (
	"testing"
)

// LastIndex returns the index of the last instance of x in list, or
// -1 if x is not present. The loop condition has a fault that
// causes somes tests to fail. Change it to i >= 0 to see them pass.
func LastIndex(list []int, x int) int {
	for i := len(list) - 1; i > 0; i-- {
		if list[i] == x {
			return i
		}
	}
	return -1
}

func TestLastIndex(t *testing.T) {
	tests := []struct {
		list []int
		x    int
		want int
	}{
		{list: []int{1}, x: 1, want: 0},
		{list: []int{1, 1}, x: 1, want: 1},
		{list: []int{2, 1}, x: 2, want: 0},
		{list: []int{1, 2, 1, 1}, x: 2, want: 1},
		{list: []int{1, 1, 1, 2, 2, 1}, x: 3, want: -1},
		{list: []int{3, 1, 2, 2, 1, 1}, x: 3, want: 0},
	}
	for _, tt := range tests {
		if got := LastIndex(tt.list, tt.x); got != tt.want {
			t.Errorf("LastIndex(%v, %v) = %v, want %v", tt.list, tt.x, got, tt.want)
		}
	}
}
