package utils

import "next.orly.dev/pkg/utils/constraints"

func FastEqual[A constraints.Bytes, B constraints.Bytes](a A, b B) (same bool) {
	if len(a) != len(b) {
		return
	}
	ab := []byte(a)
	bb := []byte(b)
	for i, v := range ab {
		if v != bb[i] {
			return
		}
	}
	return true
}
