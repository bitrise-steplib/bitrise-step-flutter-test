package main

import (
	"github.com/bitrise-tools/go-steputils/stepconf"
	"github.com/kballard/go-shellquote"
)

type configParser interface {
	parseConfig() config
	parseAdditionalParams(cfg config) []string
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

func (r realConfigParser) parseAdditionalParams(cfg config) []string {
	additionalParams, err := shellquote.Split(cfg.AdditionalParams)
	if err != nil {
		r.interrupt.failWithMessage("Failed to parse additional parameters, error: %s", err)
	}
	return additionalParams
}
