package main

import (
	"bytes"
	"errors"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/log"
)

const testProjectLocation = "foo/bar"

type testResult struct {
	failedMessage       string
	stepFailed          bool
	coverageExported    bool
	testResultsExported bool
	testExecuted        bool
	exportPath          string
}

type mockInterrupt struct {
	testResult *testResult
}

func (m mockInterrupt) failWithMessage(msg string, _ ...interface{}) {
	if m.testResult.failedMessage == "" {
		m.testResult.failedMessage = msg
	}
	m.testResult.stepFailed = true
}

func (m mockInterrupt) fail() {
	m.testResult.stepFailed = true
}

type mockParser struct {
}

func (m mockParser) parseConfig() config {
	return config{GenerateCodeCoverageFiles: true}
}

func (m mockParser) parseAdditionalParams(string) []string {
	return []string{}
}

func (m mockParser) expandTestsPathPattern(string, string) []string {
	return []string{}
}

type mockCommand struct {
	command.Command
	failWait bool
}

func (m mockCommand) Start() error {
	return nil
}

func (m mockCommand) Wait() error {
	if m.failWait {
		return errors.New("command failed")
	}
	return nil
}

func (m mockCommand) PrintableCommandArgs() string {
	return ""
}

func failingCmd() command.Command {
	return mockCommand{failWait: true}
}

func successCmd() command.Command {
	return mockCommand{failWait: false}
}

type testWrapperExecutor struct {
	realTestExecutor testExecutor
	realExport       bool
	testResult       *testResult
}

func (t testWrapperExecutor) executeTest(cfg config, additionalParams []string) (bytes.Buffer, bool) {
	return t.realTestExecutor.executeTest(cfg, additionalParams)
}

func (t testWrapperExecutor) exportTestResults(cfg config, b bytes.Buffer) {
	if t.realExport {
		t.realTestExecutor.exportTestResults(cfg, b)
	} else {
		t.testResult.testResultsExported = true
		t.testResult.coverageExported = true
	}
}

type testCommandBuilder struct {
	testFails             bool
	fileReporterSupported bool
}

func (t testCommandBuilder) supportsFileReporter() bool {
	return t.fileReporterSupported
}

func (t testCommandBuilder) buildTestCmd(bool, string, []string, *command.Opts) command.Command {
	if t.testFails {
		return failingCmd()
	}
	return successCmd()
}

func (t testCommandBuilder) buildJunitCmd(config, string, *command.Opts) command.Command {
	return successCmd()
}

func (t testCommandBuilder) buildLegacyTestCmd(bool, []string, *command.Opts) command.Command {
	if t.testFails {
		return failingCmd()
	}
	return successCmd()
}

func (t testCommandBuilder) buildLegacyJunitCmd(config, *command.Opts) command.Command {
	return successCmd()
}

func setupFailingUnitTestsExecutor(interrupt interrupt, testResult *testResult) {
	test = testWrapperExecutor{realTestExecutor: realTestExecutor{
		interrupt:      interrupt,
		logger:         log.NewLogger(),
		commandBuilder: testCommandBuilder{testFails: true},
		testExporter:   mockTestExporter{testResult: testResult},
	}, testResult: testResult}
}

func setupFailingFileReporterExecutor(interrupt interrupt, testResult *testResult) {
	test = testWrapperExecutor{realTestExecutor: realTestExecutor{
		interrupt:      interrupt,
		logger:         log.NewLogger(),
		commandBuilder: testCommandBuilder{testFails: true, fileReporterSupported: true},
		testExporter:   mockTestExporter{testResult: testResult},
	}, testResult: testResult}
}

type mockTestExporter struct {
	testResult *testResult
}

func (m mockTestExporter) exportCoverage(projectLocation string) {
	m.testResult.coverageExported = true
}

func (m mockTestExporter) copyBufferToDeployPath(bytes.Buffer) string {
	return ""
}

func (m mockTestExporter) exportDeployPath(string) {}

func (m mockTestExporter) exportTestResultsToResultPath(_ config, testResultPath string) {
	m.testResult.exportPath = testResultPath
}
