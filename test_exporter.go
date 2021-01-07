package main

import (
	"bytes"
	"github.com/bitrise-io/testresultexport/testresultexport"
	"github.com/bitrise-tools/go-steputils/tools"
)

type testExporter interface {
	copyBufferToDeployPath(jsonBuffer bytes.Buffer) string
	exportDeployPath(testResultDeployPath string)
	exportTestResultsToResultPath(cfg config, testResultPath string)
}

type realTestExporter struct {
	interrupt interrupt
}

func (r realTestExporter) copyBufferToDeployPath(jsonBuffer bytes.Buffer) string {
	return copyBufferToDeployDir(jsonBuffer.Bytes(), testResultJSONFileName, r.interrupt)
}

func (r realTestExporter) exportDeployPath(testResultDeployPath string) {
	if err := tools.ExportEnvironmentWithEnvman("BITRISE_FLUTTER_TESTRESULT_PATH", testResultDeployPath); err != nil {
		r.interrupt.failWithMessage("Failed to export: BITRISE_FLUTTER_TESTRESULT_PATH, error: %s", err)
	}
}

func (r realTestExporter) exportTestResultsToResultPath(cfg config, testResultPath string) {
	exporter := testresultexport.NewExporter(cfg.TestResultsDir)
	if err := exporter.ExportTest(testName, testResultPath); err != nil {
		r.interrupt.failWithMessage("Failed to export test result: %s", err)
	}
}
