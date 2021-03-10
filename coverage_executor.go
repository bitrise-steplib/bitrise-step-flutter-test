package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-steputils/tools"
)

const (
	coverageFileName     = "flutter_coverage_lcov.info"
	coverageRelativePath = "./coverage/lcov.info"
)

type coverageExecutor interface {
	executeCoverage(cfg config, additionalParams []string) bool
	exportCoverage(projectLocation string)
}

type realCoverageExecutor struct {
	interrupt      interrupt
	commandBuilder commandBuilder
}

func (r realCoverageExecutor) executeCoverage(cfg config, additionalParams []string) bool {
	coverageCmd := r.commandBuilder.buildCoverageCmd(additionalParams)
	coverageCmdModel := coverageCmd.toModel().
		SetDir(cfg.ProjectLocation)

	fmt.Println()
	log.Infof("Rerunning test command to generate coverage data")
	fmt.Println()
	log.Donef("$ %s", coverageCmdModel.PrintableCommandArgs())
	fmt.Println()

	if err := coverageCmdModel.Run(); err != nil {
		log.Errorf("Completing coverage command failed, error: %s", err)
		return true
	}
	return false
}

func (r realCoverageExecutor) exportCoverage(projectLocation string) {
	coverageDeployPath := copyToDeployDir(path.Join(projectLocation, coverageRelativePath), coverageFileName, r.interrupt)
	if err := tools.ExportEnvironmentWithEnvman("BITRISE_FLUTTER_COVERAGE_PATH", coverageDeployPath); err != nil {
		r.interrupt.failWithMessage("Failed to export: BITRISE_FLUTTER_COVERAGE_PATH, error: %s", err)
	}
}

func copyToDeployDir(logPth string, logFileName string, interrupt interrupt) string {
	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
	if deployDir == "" {
		interrupt.failWithMessage("no BITRISE_DEPLOY_DIR found")
	}
	deployPth := filepath.Join(deployDir, logFileName)

	if err := command.CopyFile(logPth, deployPth); err != nil {
		interrupt.failWithMessage("failed to copy `%s` info file from (%s) to (%s), error: %s", logFileName, logPth, deployPth, err)
	}
	return deployPth
}
