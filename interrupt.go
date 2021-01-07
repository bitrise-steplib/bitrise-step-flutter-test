package main

import (
	"github.com/bitrise-io/go-utils/log"
	"os"
)

type interrupt interface {
	failWithMessage(msg string, args ...interface{})
	fail()
}

type realInterrupt struct{}

func (r realInterrupt) failWithMessage(msg string, args ...interface{}) {
	log.Errorf(msg, args...)
	r.fail()
}

func (r realInterrupt) fail() {
	os.Exit(1)
}
