// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/debugshell"
)

// makeTerminalSane puts the terminal into a canonical, echoing state and returns
// a function restoring the previous settings.
//
// tcell drives the console in raw mode with echo off. Suspending the tview
// application is not enough to undo that for a child process: the shell would
// run on a terminal that echoes nothing, so an operator sees their typing
// disappear even though the shell is working. Reset it explicitly.
func makeTerminalSane(fd int) (func(), error) {
	previous, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, fmt.Errorf("failed to read terminal settings: %w", err)
	}

	sane := *previous

	sane.Iflag |= unix.BRKINT | unix.ICRNL | unix.IXON
	sane.Oflag |= unix.OPOST | unix.ONLCR
	sane.Cflag |= unix.CREAD
	sane.Lflag |= unix.ISIG | unix.ICANON | unix.ECHO | unix.ECHOE | unix.ECHOK | unix.IEXTEN

	sane.Cc[unix.VINTR] = 0x03  // Ctrl+C
	sane.Cc[unix.VQUIT] = 0x1c  // Ctrl+\
	sane.Cc[unix.VERASE] = 0x7f // backspace
	sane.Cc[unix.VKILL] = 0x15  // Ctrl+U
	sane.Cc[unix.VEOF] = 0x04   // Ctrl+D
	sane.Cc[unix.VSUSP] = 0x1a  // Ctrl+Z
	sane.Cc[unix.VMIN] = 1
	sane.Cc[unix.VTIME] = 0

	if err = unix.IoctlSetTermios(fd, unix.TCSETS, &sane); err != nil {
		return nil, fmt.Errorf("failed to apply terminal settings: %w", err)
	}

	return func() {
		unix.IoctlSetTermios(fd, unix.TCSETS, previous) //nolint:errcheck
	}, nil
}

// runShell runs an interactive shell wired to the dashboard's terminal.
//
// It returns once the shell exits, so the caller can restore the dashboard.
func runShell() error {
	argv, err := debugshell.Find()
	if err != nil {
		return err
	}

	restore, err := makeTerminalSane(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}

	defer restore()

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "/"
	cmd.Env = []string{
		"PATH=" + debugshell.Path,
		"HOME=/",
		"TERM=linux",
		"PS1=talos# ",
	}

	return cmd.Run()
}

// suspendToShell suspends the dashboard UI and hands the terminal to a shell.
//
// The dashboard is restored once the shell exits.
func (d *Dashboard) suspendToShell() {
	d.app.Suspend(func() {
		// clear the screen so the shell does not start on top of the UI
		fmt.Print("\033[2J\033[H")

		fmt.Println("Talos debug shell.")
		fmt.Println("Type 'exit' or press Ctrl+D to return to the dashboard.")
		fmt.Println()

		if err := runShell(); err != nil {
			fmt.Printf("\ndebug shell failed: %v\n", err)
			fmt.Println("press Enter to return to the dashboard")
			fmt.Fscanln(os.Stdin) //nolint:errcheck
		}
	})
}
