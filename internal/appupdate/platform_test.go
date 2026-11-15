package appupdate

import "testing"

func TestPlatformReleaseNeverInstallsAnotherPlatform(t *testing.T) {
	r := fixtureRelease("0.2.0", false)
	c, err := selectPlatformRelease([]release{r}, "0.1.0", "windows", "amd64")
	if err != nil || c == nil || c.Archive.Name != r.Assets[0].Name {
		t.Fatalf("Windows release: %#v %v", c, err)
	}
	for _, platform := range [][2]string{{"windows", "arm64"}, {"darwin", "arm64"}, {"linux", "amd64"}} {
		if _, err := selectPlatformRelease([]release{r}, "0.1.0", platform[0], platform[1]); err == nil {
			t.Fatalf("unsupported desktop %s/%s appeared up to date", platform[0], platform[1])
		}
	}
	r.Assets[0].Name = "other-platform.tar.gz"
	if c, err := selectPlatformRelease([]release{r}, "0.1.0", "windows", "amd64"); err != nil || c != nil {
		t.Fatalf("foreign package selected on Windows: %#v %v", c, err)
	}
}
