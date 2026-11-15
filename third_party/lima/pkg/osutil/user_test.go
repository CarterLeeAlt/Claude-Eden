// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package osutil

import (
	"bytes"
	"os/user"
	"path"
	"testing"

	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

const limaVersion = "1.0.0"

// "admin" is a reserved username in 1.0.0
func TestLimaUserAdminNew(t *testing.T) {
	user := guestUserForHost(user.User{Username: "admin", Uid: "501", Gid: "20"}, "linux", limaVersion, false, nil)
	assert.Equal(t, user.Username, fallbackUser)
}

// "admin" is allowed in older instances
func TestLimaUserAdminOld(t *testing.T) {
	user := guestUserForHost(user.User{Username: "admin", Uid: "501", Gid: "20"}, "linux", "0.23.0", false, nil)
	assert.Equal(t, user.Username, "admin")
}

// An empty version marks an instance created before Lima v0.20, which keeps the old home
func TestLimaUserEmptyVersionIsOld(t *testing.T) {
	user := guestUserForHost(user.User{Username: fallbackUser, Uid: "501", Gid: "20"}, "linux", "", false, nil)
	assert.Equal(t, user.HomeDir, "/home/{{.User}}.linux")
}

func TestLimaUserInvalid(t *testing.T) {
	user := guestUserForHost(user.User{Username: "use@example.com", Uid: "501", Gid: "20"}, "linux", limaVersion, false, nil)
	assert.Equal(t, user.Username, fallbackUser)
}

func TestLimaUserUid(t *testing.T) {
	user := guestUserForHost(user.User{Username: "clouduser", Uid: "2345", Gid: "3456"}, "linux", limaVersion, false, nil)
	assert.Equal(t, user.Uid, "2345")
}

func TestLimaUserGid(t *testing.T) {
	user := guestUserForHost(user.User{Username: "clouduser", Uid: "2345", Gid: "3456"}, "linux", limaVersion, false, nil)
	assert.Equal(t, user.Gid, "3456")
}

func TestLimaHomeDir(t *testing.T) {
	user := guestUserForHost(user.User{Username: fallbackUser, Uid: "501", Gid: "20"}, "linux", limaVersion, false, nil)
	// check for absolute unix path (/home)
	assert.Assert(t, path.IsAbs(user.HomeDir), user.HomeDir)
}

func TestWindowsGuestIdentityDoesNotReuseHostAccount(t *testing.T) {
	var logs bytes.Buffer
	logger := logrus.StandardLogger()
	saved := logger.Out
	logger.SetOutput(&logs)
	t.Cleanup(func() { logger.SetOutput(saved) })
	for _, name := range []string{`DOMAIN\User Name`, `主机\开发 用户`, "admin", "developer"} {
		host := user.User{Username: name, Name: "Host account", Uid: "S-1-5-21-1001", Gid: "S-1-5-21-513", HomeDir: `C:\Users\Host User`}
		original := host
		guest := guestUserForHost(host, "windows", "2.3.0", true, nil)
		assert.Equal(t, guest.Username, "developer")
		assert.Equal(t, guest.Name, "Development user")
		assert.Equal(t, guest.Uid, "1000")
		assert.Equal(t, guest.Gid, "1000")
		assert.Equal(t, guest.HomeDir, "/home/{{.User}}.guest")
		guest.Username, guest.Uid, guest.Gid = "changed", "2", "3"
		assert.DeepEqual(t, host, original)
	}
	assert.Equal(t, logs.String(), "")
}

func TestLimaUserDoesNotModifyCachedHostIdentity(t *testing.T) {
	before := *currentUser
	guest := LimaUser(t.Context(), "2.3.0", false, nil)
	guest.Username, guest.Uid, guest.Gid, guest.HomeDir = "changed", "2", "3", "/changed"
	assert.DeepEqual(t, *currentUser, before)
	again := LimaUser(t.Context(), "2.3.0", false, nil)
	assert.Assert(t, again.Username != "changed")
	assert.Assert(t, again.HomeDir != "/changed")
}

func TestPOSIXGuestIdentityPreservesHostAccount(t *testing.T) {
	for _, name := range []string{"clouduser", "use@example.com", "admin"} {
		host := user.User{Username: name, Name: "Cloud user", Uid: "2345", Gid: "3456", HomeDir: "/srv/users/clouduser"}
		original := host
		guest := guestUserForHost(host, "linux", "2.3.0", false, nil)
		want := name
		if name != "clouduser" {
			want = fallbackUser
		}
		assert.Equal(t, guest.Username, want)
		assert.Equal(t, guest.Name, host.Name)
		assert.Equal(t, guest.Uid, host.Uid)
		assert.Equal(t, guest.Gid, host.Gid)
		guest.Username, guest.HomeDir = "changed", "/changed"
		assert.DeepEqual(t, host, original)
	}
}
