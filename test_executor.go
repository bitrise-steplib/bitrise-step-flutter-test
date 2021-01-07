package main

import (
	"bytes"
	"fmt"
	"github.com/bitrise-io/go-utils/log"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	testName               = "Flutter test results"
	testResultFileName     = "flutter_junit_test_results.xml"
	testResultJSONFileName = "flutter_json_test_results.json"
)

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

	testCmd := r.commandBuilder.buildTestCmd(additionalParams)
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
}

func copyBufferToDeployDir(buffer []byte, logFileName string, interrupt interrupt) string {
	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
	if deployDir == "" {
		interrupt.failWithMessage("no BITRISE_DEPLOY_DIR found")
	}
	deployPth := filepath.Join(deployDir, logFileName)

	if err := ioutil.WriteFile(deployPth, buffer, 0664); err != nil {
		interrupt.failWithMessage("failed to write buffer to (%s), error: %s", deployPth, err)
	}
	return deployPth
}
