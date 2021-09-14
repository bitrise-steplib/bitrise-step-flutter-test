package main

import (
	"bytes"
	"errors"

	"github.com/bitrise-io/go-utils/command"
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

type mockCommandWrapper struct {
	failWait bool
}

func (m mockCommandWrapper) start() error {
	return nil
}

func (m mockCommandWrapper) wait() error {
	if m.failWait {
		return errors.New("command failed")
	}
	return nil
}

func (m mockCommandWrapper) toModel() *command.Model {
	return command.New("")
}

func failingCmd() commandWrapper {
	return mockCommandWrapper{failWait: true}
}

func successCmd() commandWrapper {
	return mockCommandWrapper{failWait: false}
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
	testFails bool
}

func (t testCommandBuilder) buildTestCmd(generateCoverage bool, additionalParams []string) commandWrapper {
	if t.testFails {
		return failingCmd()
	}
	return successCmd()
}

func (t testCommandBuilder) buildJunitCmd(config) commandWrapper {
	return successCmd()
}

func setupFailingUnitTestsExecutor(interrupt interrupt, testResult *testResult) {
	test = testWrapperExecutor{realTestExecutor: realTestExecutor{
		interrupt:      interrupt,
		commandBuilder: testCommandBuilder{testFails: true},
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
