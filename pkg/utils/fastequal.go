package utils

func FastEqual[A string | []byte, B string | []byte](a A, b B) (same bool) {
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
