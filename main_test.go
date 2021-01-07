package main

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestResultsExportedAndCoverageRunWhenTestExecutionFails(t *testing.T) {
	// Arrange
	result := testResult{}
	mi := mockInterrupt{testResult: &result}
	ir = mi
	parser = mockParser{}
	setupFailingUnitTestsExecutors(ir, &result)

	// Act
	main()

	// Assert
	assert.Equal(t, result.testResultsExported, true)
	assert.Equal(t, result.coverageExecuted, true)
	assert.Equal(t, result.coverageExported, true)
	assert.Equal(t, result.failedMessage, "")
	assert.Equal(t, result.stepFailed, true)
}

func TestCoverageExportedWhenCoverageExecutionFails(t *testing.T) {
	// Arrange
	result := testResult{}
	mi := mockInterrupt{testResult: &result}
	ir = mi
	parser = mockParser{}
	setupFailingCoverageExecutors(ir, &result)

	// Act
	main()

	// Assert
	assert.Equal(t, result.testExecuted, true)
	assert.Equal(t, result.testResultsExported, true)
	assert.Equal(t, result.coverageExported, true)
	assert.Equal(t, result.failedMessage, "")
	assert.Equal(t, result.stepFailed, true)
}

func TestResultsAreExportedFromNonRootProject(t *testing.T) {
	// Arrange
	result := testResult{}
	test := testWrapperExecutor{realTestExecutor: realTestExecutor{testExporter: mockTestExporter{testResult: &result},
	}, realExport: true}

	// Act
	test.exportTestResults(config{ProjectLocation: testProjectLocation}, bytes.Buffer{})

	// Assert
	assert.Equal(t, result.exportPath, testProjectLocation+"/"+testResultFileName)
}
