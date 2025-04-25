package util

func IsEmpty[T any](s []T) bool {
	return len(s) == 0
}
