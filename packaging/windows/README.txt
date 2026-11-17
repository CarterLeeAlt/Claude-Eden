Claude Eden for Windows 10 (x64)

Install Claude Eden for the current Windows user. On the Shortcuts page, choose a
desktop shortcut and Start menu entries. Both are selected on a fresh install.
Upgrades remember the selection. Open Claude Eden from either selected location.
For a first installation, the default is D:\Eden when D: is a local fixed
drive, or %LOCALAPPDATA%\Programs\Eden otherwise. Upgrades keep the existing
installation folder. You can choose another location in the installer.
No Go, Node.js, QEMU, OpenSSH or Clash installation is required.
The installer includes the Microsoft WebView2 offline runtime installer.

This x64 package requires Windows 10 on an Intel/AMD 64-bit processor and the
ConPTY terminal APIs, introduced by Microsoft in Windows 10 version 1809.
Startup checks the required APIs. The local Debian 13 guest also requires
hardware virtualization and Windows Hypervisor Platform.
QEMU's upstream WHPX testing on x64 starts at Windows 10 version 2004;
earlier versions require runtime validation on the target computer.
Enable Windows Hypervisor Platform in "Turn Windows features on or off", then
restart Windows if Claude Eden reports that the hypervisor is unavailable.
Windows on ARM and 32-bit Windows are not supported by this package.

In Claude Eden, choose an environment data folder before starting the environment if
you want the Debian 13 image, cache and virtual disks on another drive. Configure
your own proxy and start the environment to download the upstream Debian 13 image and tools.
Proxy service and Claude account are not included. Changes to shared projects
affect the original files.

Updates preserve the Claude Eden configuration, virtual disk and project files.
Projects and links inside the installation folder are retained. Modified files
that conflict with the new program, and unrecognized libraries in program load
directories, are kept in Eden-preserved-files inside that folder. Changes made
during installation can also be retained in its late-* subdirectories.
If recovery is interrupted, a .eden-installation.json record beside the
application folder identifies the retained installation, staged and backup
folders. Keep the record and those folders until recovery is complete.
Automatic updates stop application launch while recovery remains incomplete.
Stop the environment and quit Claude Eden before running an installer manually.
In-app updates stop the environment after your confirmation, verify the new
installer and publisher, and restart the application after installation.
If installation fails while the original program is intact, Claude Eden reopens it
without replacing its directory. If the new version cannot become ready, Claude Eden
attempts to restore the previous program and keeps the latest personal files.
Unsigned development builds cannot install automatic updates.

Uninstall using Windows Settings > Apps > Apps & features.
Uninstall removes application files and shortcuts.
It preserves user data in %USERPROFILE%\.eden and all shared projects.
If personal files remain inside the installation folder, the folder remains
available for a later installation.
Microsoft WebView2 is a shared runtime and is not removed by Claude Eden uninstall.

Getting started: https://github.com/CarterLeeAlt/Claude-Eden/blob/main/docs/windows.md
Issues: https://github.com/CarterLeeAlt/Claude-Eden/issues
