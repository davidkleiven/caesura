package pkg

func PanicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}

func RemoveDuplicates[T comparable](input []T) []T {
	seen := make(map[T]struct{})
	result := make([]T, 0, len(input))

	for _, v := range input {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}
