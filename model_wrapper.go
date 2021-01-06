package main

import "github.com/bitrise-io/go-utils/command"

type modelWrapper interface {
	PrintableCommandArgs() string
	Run() error
}

type realModelWrapper struct {
	model *command.Model
}

func (r realModelWrapper) Run() error {
	return r.model.Run()
}

func (r realModelWrapper) PrintableCommandArgs() string {
	return r.model.PrintableCommandArgs()
}
