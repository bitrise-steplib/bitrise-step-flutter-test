package main

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-android/v2/testresult/junitxml"
	"github.com/bitrise-io/go-steputils/v2/testattachment"
	"github.com/bitrise-io/go-steputils/v2/testreport"
	"github.com/bitrise-io/go-utils/v2/log"
)

// exportConventionAttachments copies the files under root that are named after a test case of the
// JUnit XML at junitPath into reportDir, where the deploy step links them to their test case.
// Problems are logged as warnings: the test result itself is already exported.
func exportConventionAttachments(logger log.Logger, collector testattachment.Collector, root, deployDir, junitPath, reportDir string) {
	report, err := readJUnitReport(junitPath)
	if err != nil {
		logger.Warnf("Failed to read test cases, attachments are not matched by file name: %s", err)
		return
	}

	if deployDir == "" {
		logger.Warnf("BITRISE_TEST_DEPLOY_DIR is not set, attachments exported by earlier steps are not filtered out")
	}

	result, err := collector.Collect(absPath(root), deployDir, testattachment.NewIndex(&report))
	if err != nil {
		logger.Warnf("Failed to collect attachments matched by file name: %s", err)
		return
	}
	if result.GitCheckErr != nil {
		logger.Warnf("Failed to check which files are tracked by git, committed files are not filtered out: %s", result.GitCheckErr)
	}
	logSkippedAttachments(logger, result.Skipped)

	if len(result.Candidates) == 0 {
		return
	}

	logger.Donef("Exporting %d attachments matched by file name.", len(result.Candidates))
	for _, failed := range collector.CopyToReport(reportDir, result.Candidates) {
		logger.Warnf("Failed to export attachment %s: %s", failed.Path, failed.Reason)
	}
}

func readJUnitReport(path string) (testreport.TestReport, error) {
	var converter junitxml.Converter
	if !converter.Detect([]string{path}) {
		return testreport.TestReport{}, fmt.Errorf("%s is not a JUnit XML", path)
	}
	return converter.Convert()
}

// Likely naming mistakes are warnings. The rest, like a file of another report or a committed
// file, are expected under a project folder and only logged in debug mode.
func logSkippedAttachments(logger log.Logger, skipped []testattachment.Skipped) {
	for _, s := range skipped {
		switch {
		case errors.Is(s.Reason, testattachment.ErrDuplicateName),
			errors.Is(s.Reason, testattachment.ErrAmbiguousTest),
			errors.Is(s.Reason, testattachment.ErrUnknownRun),
			errors.Is(s.Reason, testattachment.ErrMissingLabel):
			logger.Warnf("Skipping attachment %s: %s", s.Path, s.Reason)
		default:
			logger.Debugf("Skipping attachment %s: %s", s.Path, s.Reason)
		}
	}
}

func absPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return filepath.Clean(path)
}
