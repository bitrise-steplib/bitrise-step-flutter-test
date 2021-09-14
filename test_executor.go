package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/bitrise-io/go-utils/log"
)

const testResultFileName = "flutter_junit_test_results.xml"

type testExecutor interface {
	executeTest(cfg config, additionalParams []string) (bytes.Buffer, bool)
	exportTestResults(cfg config, jsonBuffer bytes.Buffer)
}

type realTestExecutor struct {
	interrupt      interrupt
	commandBuilder commandBuilder
	testExporter   testExporter
}

func (r realTestExecutor) executeTest(cfg config, additionalParams []string) (bytes.Buffer, bool) {
	var jsonBuffer bytes.Buffer
	pr, pw := io.Pipe()
	testCmdWriter := io.MultiWriter(pw, &jsonBuffer)

	testCmd := r.commandBuilder.buildTestCmd(cfg.GenerateCodeCoverageFiles, additionalParams)
	junitCmd := r.commandBuilder.buildJunitCmd(cfg)

	testExecutionFailed := false

	testCmdModel := testCmd.toModel().
		SetStdout(testCmdWriter).
		SetStderr(os.Stderr).
		SetDir(cfg.ProjectLocation)

	junitCmdModel := junitCmd.toModel().
		SetStdin(pr).
		SetStdout(os.Stdout).
		SetStderr(os.Stderr).
		SetDir(cfg.ProjectLocation)

	fmt.Println()
	log.Donef("$ %s | %s", testCmdModel.PrintableCommandArgs(), junitCmdModel.PrintableCommandArgs())
	fmt.Println()

	if err := testCmd.start(); err != nil {
		r.interrupt.failWithMessage("Running command failed, error: %s", err)
	}

	if err := junitCmd.start(); err != nil {
		r.interrupt.failWithMessage("Converting test results to junit format failed, error: %s", err)
	}

	if err := testCmd.wait(); err != nil {
		log.Errorf("Completing test command failed, error: %s", err)
		testExecutionFailed = true
	}

	if err := pw.Close(); err != nil {
		r.interrupt.failWithMessage("Closing pipe failed, error: %s", err)
	}

	if err := junitCmd.wait(); err != nil {
		r.interrupt.failWithMessage("Completing conversion command failed, error: %s", err)
	}
	return jsonBuffer, testExecutionFailed
}

func (r realTestExecutor) exportTestResults(cfg config, jsonBuffer bytes.Buffer) {
	testResultDeployPath := r.testExporter.copyBufferToDeployPath(jsonBuffer)
	r.testExporter.exportDeployPath(testResultDeployPath)

	testResultPath := cfg.ProjectLocation + "/" + testResultFileName

	r.testExporter.exportTestResultsToResultPath(cfg, testResultPath)

	if cfg.GenerateCodeCoverageFiles {
		r.testExporter.exportCoverage(cfg.ProjectLocation)
	}
}
