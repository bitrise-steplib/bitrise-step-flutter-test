package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

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
	if hasFileReporterArg(additionalParams) {
		r.interrupt.failWithMessage("Process config: passing --file-reporter in Additional parameters is not supported; the step sets it up internally.")
		return bytes.Buffer{}, true
	}

	if r.commandBuilder.supportsFileReporter() {
		return r.executeTestWithFileReporter(cfg, additionalParams)
	}

	log.Warnf("`flutter test` does not support --file-reporter (Flutter < 3.10); the log will show the raw machine JSON stream. Upgrade to Flutter 3.10 or newer for human-readable test output.")
	return r.executeTestLegacy(cfg, additionalParams)
}

// executeTestWithFileReporter runs `flutter test` with --file-reporter so stdout keeps the
// normal, human-readable reporter output, then converts the JSON report file to JUnit XML.
func (r realTestExecutor) executeTestWithFileReporter(cfg config, additionalParams []string) (bytes.Buffer, bool) {
	jsonReportFile, err := os.CreateTemp("", "flutter_json_test_results_*.json")
	if err != nil {
		r.interrupt.failWithMessage("Run: failed to create temporary test report file: %s", err)
	}
	jsonReportPath := jsonReportFile.Name()
	jsonReportFile.Close()
	defer os.Remove(jsonReportPath)

	testCmd := r.commandBuilder.buildTestCmd(cfg.GenerateCodeCoverageFiles, jsonReportPath, additionalParams)
	testCmdModel := testCmd.toModel().
		SetStdout(os.Stdout).
		SetStderr(os.Stderr).
		SetDir(cfg.ProjectLocation)

	fmt.Println()
	log.Donef("$ %s", testCmdModel.PrintableCommandArgs())
	fmt.Println()

	testExecutionFailed := false
	if err := testCmd.start(); err != nil {
		r.interrupt.failWithMessage("Run: test command failed: %s", err)
	}

	// Don't abort on test failures: we still want to convert and export whatever
	// results were produced.
	if err := testCmd.wait(); err != nil {
		log.Errorf("Run: completing test command failed: %s", err)
		testExecutionFailed = true
	}

	// Read back the JSON report for the JUnit conversion and the JSON output export.
	// A missing/unreadable report (e.g. flutter crashed before writing it, or a mocked
	// command in unit tests) is not fatal: continue with an empty buffer.
	var jsonBuffer bytes.Buffer
	if data, err := os.ReadFile(jsonReportPath); err != nil {
		log.Warnf("Run: could not read test result JSON (%s): %s", jsonReportPath, err)
	} else {
		jsonBuffer.Write(data)
	}

	junitCmd := r.commandBuilder.buildJunitCmd(cfg, jsonReportPath)
	junitCmdModel := junitCmd.toModel().
		SetStdout(os.Stdout).
		SetStderr(os.Stderr).
		SetDir(cfg.ProjectLocation)

	fmt.Println()
	log.Donef("$ %s", junitCmdModel.PrintableCommandArgs())
	fmt.Println()

	if err := junitCmd.start(); err != nil {
		r.interrupt.failWithMessage("Run: converting test results to junit format failed: %s", err)
	}

	if err := junitCmd.wait(); err != nil {
		r.interrupt.failWithMessage("Run: completing conversion command failed: %s", err)
	}

	return jsonBuffer, testExecutionFailed
}

// executeTestLegacy is the pre-3.10 fallback: it pipes `flutter test --machine` stdout
// straight into tojunit. The build log therefore shows the raw machine JSON stream
// (the step's original behavior).
func (r realTestExecutor) executeTestLegacy(cfg config, additionalParams []string) (bytes.Buffer, bool) {
	var jsonBuffer bytes.Buffer
	pr, pw := io.Pipe()
	testCmdWriter := io.MultiWriter(pw, &jsonBuffer)

	testCmd := r.commandBuilder.buildLegacyTestCmd(cfg.GenerateCodeCoverageFiles, additionalParams)
	junitCmd := r.commandBuilder.buildLegacyJunitCmd(cfg)

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
		r.interrupt.failWithMessage("Run: test command failed: %s", err)
	}

	if err := junitCmd.start(); err != nil {
		r.interrupt.failWithMessage("Run: converting test results to junit format failed: %s", err)
	}

	// Wait for testCmd concurrently and close pw when done so junitCmd receives EOF.
	// This prevents a deadlock where junitCmd exits early, pw.Write() blocks because
	// pr is no longer being read, and testCmd.Wait() never returns.
	testErrCh := make(chan error, 1)
	go func() {
		testErrCh <- testCmd.wait()
		pw.Close()
	}()

	junitErr := junitCmd.wait()
	pr.Close() // unblocks pw.Write() if junitCmd exited before testCmd finished

	if junitErr != nil {
		r.interrupt.failWithMessage("Run: completing conversion command failed: %s", junitErr)
	}

	if err := <-testErrCh; err != nil {
		log.Errorf("Run: completing test command failed: %s", err)
		testExecutionFailed = true
	}

	return jsonBuffer, testExecutionFailed
}

// hasFileReporterArg reports whether the user passed their own --file-reporter flag
// (either "--file-reporter=<value>" or "--file-reporter <value>"). The step manages
// --file-reporter itself, so a user-supplied one is rejected.
func hasFileReporterArg(params []string) bool {
	const flag = "--file-reporter"
	for _, p := range params {
		if p == flag || strings.HasPrefix(p, flag+"=") {
			return true
		}
	}
	return false
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
