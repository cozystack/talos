// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/console"
	"github.com/siderolabs/talos/internal/pkg/debugshell"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// shellRunning guards against spawning a second shell: the console can be
// switched back to the dashboard while the shell is still alive.
var shellRunning atomic.Bool

// makeSane puts a terminal into a canonical, echoing state.
//
// A freshly opened virtual console is already sane, but be explicit rather than
// depend on whatever the kernel or a previous user left behind.
func makeSane(fd int) error {
	termios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return fmt.Errorf("failed to read terminal settings: %w", err)
	}

	termios.Iflag |= unix.BRKINT | unix.ICRNL | unix.IXON
	termios.Oflag |= unix.OPOST | unix.ONLCR
	termios.Cflag |= unix.CREAD
	termios.Lflag |= unix.ISIG | unix.ICANON | unix.ECHO | unix.ECHOE | unix.ECHOK | unix.IEXTEN

	termios.Cc[unix.VINTR] = 0x03  // Ctrl+C
	termios.Cc[unix.VQUIT] = 0x1c  // Ctrl+\
	termios.Cc[unix.VERASE] = 0x7f // backspace
	termios.Cc[unix.VKILL] = 0x15  // Ctrl+U
	termios.Cc[unix.VEOF] = 0x04   // Ctrl+D
	termios.Cc[unix.VSUSP] = 0x1a  // Ctrl+Z
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0

	if err = unix.IoctlSetTermios(fd, unix.TCSETS, termios); err != nil {
		return fmt.Errorf("failed to apply terminal settings: %w", err)
	}

	return nil
}

// runShellOnTTY runs an interactive shell on its own virtual console.
//
// The shell deliberately does not share the dashboard's terminal: tcell drives
// that one in raw mode with echo off and keeps re-applying it, so a shell there
// receives keystrokes but echoes nothing - it cannot be fixed even with
// `stty sane` from the inside. A separate console has none of that contention.
func runShellOnTTY() error {
	argv, err := debugshell.Find()
	if err != nil {
		return err
	}

	tty, err := os.OpenFile(fmt.Sprintf("/dev/tty%d", constants.DebugShellTTY), os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open the debug shell console: %w", err)
	}

	defer tty.Close() //nolint:errcheck

	if err = makeSane(int(tty.Fd())); err != nil {
		return err
	}

	fmt.Fprint(tty, "\033[2J\033[H")
	fmt.Fprintf(tty, "Talos debug shell on tty%d.\r\n", constants.DebugShellTTY)
	fmt.Fprintf(tty, "Type 'exit' or press Ctrl+D to return to the dashboard (Alt+F%d).\r\n\r\n", constants.DashboardTTY)

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	cmd.Dir = "/"
	cmd.Env = []string{
		"PATH=" + debugshell.Path,
		"HOME=/",
		"TERM=linux",
		"PS1=talos# ",
	}
	// Own the console: a session leader with a controlling terminal is what makes
	// the shell interactive - it enables job control and its line editor.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0,
	}

	return cmd.Run()
}

// suspendToShell hands a console over to an interactive shell.
//
// The dashboard keeps running on its own console untouched; the active console
// is switched to the shell and back when it exits.
func (d *Dashboard) suspendToShell() {
	if !shellRunning.CompareAndSwap(false, true) {
		// already running: just switch to it
		console.Switch(constants.DebugShellTTY) //nolint:errcheck

		return
	}

	go func() {
		defer shellRunning.Store(false)

		if err := console.Switch(constants.DebugShellTTY); err != nil {
			return
		}

		defer console.Switch(constants.DashboardTTY) //nolint:errcheck

		if err := runShellOnTTY(); err != nil {
			if tty, openErr := os.OpenFile(fmt.Sprintf("/dev/tty%d", constants.DebugShellTTY), os.O_RDWR, 0); openErr == nil {
				fmt.Fprintf(tty, "\r\ndebug shell failed: %v\r\n", err)
				tty.Close() //nolint:errcheck
			}
		}
	}()
}
