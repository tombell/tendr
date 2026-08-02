//go:build !darwin && !linux

package herdr

import "os/exec"

func detachProcess(command *exec.Cmd) {}
