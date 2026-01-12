package choice

// Ternary operator
func Ternary[T any](condition bool, isTrue, isFalse T) T {
	if condition == true {
		return isTrue
	}

	return isFalse
}

// Function ternary operator
func FunctionTernary[T any](condition bool, isTrue, isFalse func() T) T {
	if condition == true {
		return isTrue()
	}

	return isFalse()
}
