//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows && !zos
// +build !aix,!darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!solaris,!windows,!zos

package framework

import (
	"os/exec"
)

func registerPID(cmd *exec.Cmd) {
	// ignore
}

func execCommand(workdir, name string, args ...string) *exec.Cmd {
	cmd := exec.Command(binaryName, args...)
	cmd.Dir = workdir

	return cmd
}

func processKill(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
