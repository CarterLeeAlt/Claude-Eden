// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package filenames

// OpenSSH adds a 17-byte temporary suffix; reserve it without consuming an
// unnecessary seven bytes of the Windows AF_UNIX path budget.
const SSHSock = "s"
