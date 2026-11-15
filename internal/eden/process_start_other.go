//go:build !windows && !linux && !darwin

package eden

func processStartToken(int) string { return "" }
