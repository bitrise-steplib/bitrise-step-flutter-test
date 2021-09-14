package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
