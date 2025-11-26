package slice

import "slices"

// LastElement returns the last element of a slice of any type.
func LastElement[T any](s []T) T {
	return s[len(s)-1]
}

// Contains checks if a string slice contains a specific item.
func Contains[T comparable](slice []T, item T) bool {
	return slices.Contains(slice, item)
}
