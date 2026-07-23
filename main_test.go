package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildTestCmdUsesFileReporterNotMachine(t *testing.T) {
	builder := realCommandBuilder{}

	cmd := builder.buildTestCmd(false, "/tmp/report.json", []string{"--reporter", "expanded"})
	args := cmd.toModel().PrintableCommandArgs()

	// --machine must be absent: it forces the JSON reporter and hides human-readable output.
	assert.False(t, strings.Contains(args, "--machine"), "expected --machine to be absent, got: %s", args)
	// The JSON is written to a file instead, leaving stdout for the human-readable reporter.
	assert.True(t, strings.Contains(args, "--file-reporter=json:/tmp/report.json"), "expected --file-reporter, got: %s", args)
	// A user-supplied reporter flag is passed through and now actually takes effect.
	assert.True(t, strings.Contains(args, "expanded"), "expected additional params to be appended, got: %s", args)
}

func TestHasFileReporterArg(t *testing.T) {
	tests := []struct {
		name   string
		params []string
		want   bool
	}{
		{"absent", []string{"--reporter", "expanded"}, false},
		{"equals form", []string{"--file-reporter=json:/tmp/r.json"}, true},
		{"space form", []string{"--file-reporter", "json:/tmp/r.json"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, hasFileReporterArg(tt.params))
		})
	}
}

func TestResultsExportedWhenExecutionFails(t *testing.T) {
	// Arrange
	result := testResult{}
	mi := mockInterrupt{testResult: &result}
	ir = mi
	parser = mockParser{}
	setupFailingUnitTestsExecutor(ir, &result)

	// Act
	main()

	// Assert
	assert.Equal(t, true, result.testResultsExported)
	assert.Equal(t, true, result.coverageExported)
	assert.Equal(t, "", result.failedMessage)
	assert.Equal(t, true, result.stepFailed)
}

func TestResultsExportedWhenExecutionFailsOnFileReporterPath(t *testing.T) {
	// Arrange
	result := testResult{}
	mi := mockInterrupt{testResult: &result}
	ir = mi
	parser = mockParser{}
	setupFailingFileReporterExecutor(ir, &result)

	// Act
	main()

	// Assert: even on the modern --file-reporter path, a failing test run still
	// exports results and marks the step failed, without aborting mid-run.
	assert.Equal(t, true, result.testResultsExported)
	assert.Equal(t, true, result.coverageExported)
	assert.Equal(t, "", result.failedMessage)
	assert.Equal(t, true, result.stepFailed)
}

func TestCoverageExportedWhenExecutionFails(t *testing.T) {
	// Arrange
	result := testResult{}
	mi := mockInterrupt{testResult: &result}
	ir = mi
	parser = mockParser{}
	setupFailingUnitTestsExecutor(ir, &result)

	// Act
	main()

	// Assert
	assert.Equal(t, false, result.testExecuted)
	assert.Equal(t, true, result.testResultsExported)
	assert.Equal(t, true, result.coverageExported)
	assert.Equal(t, "", result.failedMessage)
	assert.Equal(t, true, result.stepFailed)
}

func TestResultsAreExportedFromNonRootProject(t *testing.T) {
	// Arrange
	result := testResult{}
	test := testWrapperExecutor{realTestExecutor: realTestExecutor{testExporter: mockTestExporter{testResult: &result}}, realExport: true}

	// Act
	test.exportTestResults(config{ProjectLocation: testProjectLocation}, bytes.Buffer{})

	// Assert
	assert.Equal(t, result.exportPath, testProjectLocation+"/"+testResultFileName)
}
