//go:build windows && (!eden_unsigned_fixture || production)

package desktopupdate

func Publisher(path string) (string, error) {
	return signedPublisher(path)
}
