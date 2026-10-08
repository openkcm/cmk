package transform

// ToList transforms a slice of items into their API representations.
// Parameters:
//
//	items: input slice of item
//	total: size of the slice
//	toAPI: transformation function for single object
//
// Returns transformed slice or error if transformation fails
func ToList[T any, K any](items []*T, toAPI func(T) (*K, error)) ([]K, error) {
	values := make([]K, len(items))

	for i, item := range items {
		apiResponse, err := toAPI(*item)
		if err != nil {
			return nil, err
		}

		values[i] = *apiResponse
	}

	return values, nil
}
