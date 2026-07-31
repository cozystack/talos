// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package debugshell locates the interactive debug shell shipped with an image.
//
// Stock Talos images contain no shell at all, so the debug shell is only
// available in images that bake one in - either via the debug-tools system
// extension, which installs busybox into /usr/local/bin, or via a build that
// copies a shell into the rootfs.
package debugshell

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Candidates lists shell binaries to look for, in order of preference.
var Candidates = []string{
	"/usr/local/bin/busybox",
	"/usr/local/bin/sh",
	"/bin/busybox",
	"/bin/bash",
	"/bin/sh",
}

// Path is the PATH handed to the debug shell.
//
// /usr/local/{bin,sbin} come first, as that is where system extensions land.
const Path = "/usr/local/bin:/usr/local/sbin:/bin:/sbin:/usr/bin:/usr/sbin"

// ErrNotFound is returned when no shell binary is available in the image.
var ErrNotFound = errors.New("no shell binary found in the image: install the debug-tools system extension")

// Find returns argv for the first shell binary available in the image.
//
// The shell is always asked to be interactive: it inherits the dashboard's
// session rather than being started from a login, and without -i busybox ash
// decides it is non-interactive and prints no prompt at all.
func Find() ([]string, error) {
	for _, candidate := range Candidates {
		st, err := os.Stat(candidate)
		if err != nil || st.IsDir() || st.Mode()&0o111 == 0 {
			continue
		}

		// busybox is a multi-call binary, it needs the applet name.
		if filepath.Base(candidate) == "busybox" {
			return []string{candidate, "sh", "-i"}, nil
		}

		return []string{candidate, "-i"}, nil
	}

	return nil, ErrNotFound
}

// Enabled reports whether the debug shell should be offered on the console.
//
// It is enabled whenever the image ships a shell, unless explicitly disabled
// with talos.dashboard.shell=0 on the kernel command line.
func Enabled() bool {
	if val := procfs.ProcCmdline().Get(constants.KernelParamDashboardShell).First(); val != nil {
		switch pointer.SafeDeref(val) {
		case "0", "false", "off":
			return false
		}
	}

	_, err := Find()

	return err == nil
}
