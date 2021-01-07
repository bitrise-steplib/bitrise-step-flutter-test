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
	coverageExecuted    bool
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

func (m mockParser) parseAdditionalParams(config) []string {
	return []string{}
}

type mockCoverageExecutor struct {
	testResult *testResult
}

func (m mockCoverageExecutor) executeCoverage([]string) bool {
	m.testResult.coverageExecuted = true
	return false
}

func (m mockCoverageExecutor) exportCoverage() {
	m.testResult.coverageExported = true
}

type mockTestExecutor struct {
	testResult *testResult
}

func (m mockTestExecutor) copyBufferToDeployPath(bytes.Buffer) string {
	return ""
}

func (m mockTestExecutor) exportDeployPath(string) {}

func (m mockTestExecutor) exportTestResultsToResultPath(config, string) {}

func (m mockTestExecutor) executeTest(config, []string) (bytes.Buffer, bool) {
	m.testResult.testExecuted = true
	return bytes.Buffer{}, false
}

func (m mockTestExecutor) exportTestResults(config, bytes.Buffer) {
	m.testResult.testResultsExported = true
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

type mockModelWrapper struct {
	failRun bool
}

func (m mockModelWrapper) PrintableCommandArgs() string {
	return ""
}

func (m mockModelWrapper) Run() error {
	if m.failRun {
		return errors.New("command failed")
	}
	return nil
}

func failingCmd() commandWrapper {
	return mockCommandWrapper{failWait: true}
}

func successCmd() commandWrapper {
	return mockCommandWrapper{failWait: false}
}

func failingModel() modelWrapper {
	return mockModelWrapper{failRun: true}
}

func successModel() modelWrapper {
	return mockModelWrapper{failRun: false}
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
	}
}

type coverageWrapperExecutor struct {
	realCoverageExecutor realCoverageExecutor
	testResult           *testResult
}

func (c coverageWrapperExecutor) executeCoverage(additionalParams []string) bool {
	return c.realCoverageExecutor.executeCoverage(additionalParams)
}

func (c coverageWrapperExecutor) exportCoverage() {
	c.testResult.coverageExported = true
}

type testCommandBuilder struct {
	testFails     bool
	coverageFails bool
}

func (t testCommandBuilder) buildTestCmd([]string) commandWrapper {
	if t.testFails {
		return failingCmd()
	}
	return successCmd()
}

func (t testCommandBuilder) buildJunitCmd(config) commandWrapper {
	return successCmd()
}

func (t testCommandBuilder) buildCoverageCmd([]string) modelWrapper {
	if t.coverageFails {
		return failingModel()
	}
	return successModel()
}

func setupFailingUnitTestsExecutors(interrupt interrupt, testResult *testResult) {
	coverage = mockCoverageExecutor{testResult: testResult}
	test = testWrapperExecutor{realTestExecutor: realTestExecutor{
		interrupt:      interrupt,
		commandBuilder: testCommandBuilder{testFails: true},
	}, testResult: testResult}
}

func setupFailingCoverageExecutors(interrupt interrupt, testResult *testResult) {
	coverage = coverageWrapperExecutor{realCoverageExecutor: realCoverageExecutor{
		interrupt:      interrupt,
		commandBuilder: testCommandBuilder{coverageFails: true},
	}, testResult: testResult}
	test = mockTestExecutor{testResult: testResult}
}

type mockTestExporter struct {
	testResult *testResult
}

func (m mockTestExporter) copyBufferToDeployPath(bytes.Buffer) string {
	return ""
}

func (m mockTestExporter) exportDeployPath(string) {}

func (m mockTestExporter) exportTestResultsToResultPath(_ config, testResultPath string) {
	m.testResult.exportPath = testResultPath
}
