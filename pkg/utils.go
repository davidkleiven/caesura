package pkg

import "golang.org/x/exp/constraints"

func Assert(condition bool, message string) {
	if !condition {
		panic(message)
	}
}

func Confine[T constraints.Ordered](value, min, max T) T {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func AbsInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
