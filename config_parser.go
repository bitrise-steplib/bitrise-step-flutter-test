package main

import (
	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bmatcuk/doublestar/v3"
	"github.com/kballard/go-shellquote"
	"os"
	"path/filepath"
	"strings"
)

type configParser interface {
	parseConfig() config
	parseAdditionalParams(additionalParams string) []string
	expandTestsPathPattern(projectLocation string, testsPathPattern string) []string
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

func (r realConfigParser) expandTestsPathPattern(projectLocation string, testsPathPattern string) []string {
	if testsPathPattern == "" {
		return nil
	}
	var result []string
	glob, err := doublestar.Glob(filepath.Join(projectLocation, testsPathPattern))
	if err != nil {
		log.Warnf("Couldn't expand pattern: %s: %s", testsPathPattern, err)
		return nil
	}
	for _, path := range glob {
		result = append(result, strings.TrimPrefix(path, projectLocation+string(os.PathSeparator)))
	}
	return result
}

func (r realConfigParser) parseAdditionalParams(additionalParams string) []string {
	ap, err := shellquote.Split(additionalParams)
	if err != nil {
		r.interrupt.failWithMessage("Failed to parse additional parameters, error: %s", err)
	}
	return ap
}
