package boolutil

// AnyTrue returns true if any of the provided boolean values is true.
func AnyTrue(bools ...bool) bool {
	for _, b := range bools {
		if b {
			return true
		}
	}

	return false
}
