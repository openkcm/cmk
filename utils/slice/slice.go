package slice

// LastElement returns the last element of a slice of any type.
func LastElement[T any](s []T) T {
	return s[len(s)-1]
}

// Contains checks if a string slice contains a specific item.
func Contains[T comparable](slice []T, item T) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}

	return false
}
