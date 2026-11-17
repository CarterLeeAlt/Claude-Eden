//go:build windows && eden_unsigned_fixture && !production

package desktopupdate

// Publisher is relaxed only in the headless unsigned update fixture. The
// production Windows build uses publisher_windows.go unchanged.
func Publisher(string) (string, error) {
	return "EDEN_UNSIGNED_FIXTURE", nil
}
