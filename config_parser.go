package main

import (
	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bmatcuk/doublestar/v3"
	"github.com/kballard/go-shellquote"
	"os"
)

type configParser interface {
	parseConfig() config
	parseAdditionalParams(cfg config) []string
	expandTestsPathPattern(cfg config) []string
}

type realConfigParser struct {
	interrupt interrupt
}

func (r realConfigParser) parseConfig() config {
	var cfg config
	if err := stepconf.Parse(&cfg); err != nil {
		r.interrupt.failWithMessage("Issue with input: %s", err)
	}
	return cfg
}

func (r realConfigParser) expandTestsPathPattern(cfg config) []string {
	glob, err := doublestar.Glob(cfg.ProjectLocation + string(os.PathSeparator) + cfg.TestsPathPattern)
	if err != nil {
		log.Warnf("Couldn't expand pattern: %s, cause: %s", cfg.TestsPathPattern, err)
		return nil
	}
	return glob
}

func (r realConfigParser) parseAdditionalParams(cfg config) []string {
	additionalParams, err := shellquote.Split(cfg.AdditionalParams)
	if err != nil {
		r.interrupt.failWithMessage("Failed to parse additional parameters, error: %s", err)
	}
	return additionalParams
}
