package main

import (
	"os"

	"github.com/bitrise-io/go-utils/v2/log"
)

type interrupt interface {
	failWithMessage(msg string, args ...interface{})
	fail()
}

type realInterrupt struct {
	logger log.Logger
}

func (r realInterrupt) failWithMessage(msg string, args ...interface{}) {
	r.logger.Errorf(msg, args...)
	r.fail()
}

func (r realInterrupt) fail() {
	os.Exit(1)
}
