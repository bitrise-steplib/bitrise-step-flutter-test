package main

import (
	"github.com/bitrise-io/go-utils/command"
	"os/exec"
)

type commandWrapper interface {
	start() error
	wait() error
	toModel() *command.Model
}

type realCommandWrapper struct {
	cmd *exec.Cmd
}

func (w realCommandWrapper) start() error {
	return w.cmd.Start()
}

func (w realCommandWrapper) wait() error {
	return w.cmd.Wait()
}

func (w realCommandWrapper) toModel() *command.Model {
	return command.NewWithCmd(w.cmd)
}
