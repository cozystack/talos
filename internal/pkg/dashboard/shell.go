// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/siderolabs/talos/internal/pkg/debugshell"
)

// runShell runs an interactive shell wired to the dashboard's terminal.
//
// It returns once the shell exits, so the caller can restore the dashboard.
func runShell() error {
	argv, err := debugshell.Find()
	if err != nil {
		return err
	}

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
