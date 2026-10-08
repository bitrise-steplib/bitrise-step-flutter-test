package main

import (
	"bytes"
	"os"
	"path"
	"path/filepath"

	"github.com/bitrise-io/go-steputils/v2/export"
	"github.com/bitrise-io/go-steputils/v2/testattachment"
	"github.com/bitrise-io/go-steputils/v2/testresultexport" //nolint:staticcheck // deprecated for new steps only, the package exists for this one
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
)

const (
	testName               = "Flutter test results"
	testResultJSONFileName = "flutter_json_test_results.json"
	coverageFileName       = "flutter_coverage_lcov.info"
	coverageRelativePath   = "./coverage/lcov.info"
)

type testExporter interface {
	copyBufferToDeployPath(jsonBuffer bytes.Buffer) string
	exportDeployPath(testResultDeployPath string)
	exportTestResultsToResultPath(cfg config, testResultPath string)
	exportAttachments(cfg config, testResultPath string)
	exportCoverage(projectLocation string)
}

type realTestExporter struct {
	interrupt           interrupt
	logger              log.Logger
	outputExporter      export.Exporter
	fileManager         fileutil.FileManager
	attachmentCollector testattachment.Collector
	testDeployDir       string
}

func (r realTestExporter) copyBufferToDeployPath(jsonBuffer bytes.Buffer) string {
	return copyBufferToDeployDir(jsonBuffer.Bytes(), testResultJSONFileName, r.interrupt)
}

func (r realTestExporter) exportDeployPath(testResultDeployPath string) {
	if err := r.outputExporter.ExportOutput("BITRISE_FLUTTER_TESTRESULT_PATH", testResultDeployPath); err != nil {
		r.interrupt.failWithMessage("Failed to export: BITRISE_FLUTTER_TESTRESULT_PATH, error: %s", err)
	}
	r.logger.Donef("Test results exported in JUnit format as $BITRISE_FLUTTER_TESTRESULT_PATH")
}

func (r realTestExporter) exportTestResultsToResultPath(cfg config, testResultPath string) {
	exporter := testresultexport.NewExporter(cfg.TestResultsDir, r.fileManager)
	if err := exporter.ExportTest(testName, testResultPath); err != nil {
		r.interrupt.failWithMessage("Export outputs: failed to export test result: %s", err)
	}
}

func (r realTestExporter) exportAttachments(cfg config, testResultPath string) {
	reportDir := filepath.Join(cfg.TestResultsDir, testName)
	exportConventionAttachments(r.logger, r.attachmentCollector, cfg.ProjectLocation, r.testDeployDir, testResultPath, reportDir)
}

func (r realTestExporter) exportCoverage(projectLocation string) {
	covData, err := os.ReadFile(path.Join(projectLocation, coverageRelativePath))
	if err != nil {
		r.interrupt.failWithMessage("Export outputs: failed to open %s", coverageRelativePath)
	}

	covDeployPath := copyBufferToDeployDir(covData, coverageFileName, r.interrupt)

	if err := r.outputExporter.ExportOutput("BITRISE_FLUTTER_COVERAGE_PATH", covDeployPath); err != nil {
		r.interrupt.failWithMessage("Export outputs: failed to export $BITRISE_FLUTTER_COVERAGE_PATH: %s", err)
	}

	r.logger.Donef("Test coverage file exported as $BITRISE_FLUTTER_COVERAGE_PATH")
}

func copyBufferToDeployDir(buffer []byte, logFileName string, interrupt interrupt) string {
	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
	if deployDir == "" {
		interrupt.failWithMessage("Export outputs: no $BITRISE_DEPLOY_DIR found")
	}
	deployPth := filepath.Join(deployDir, logFileName)

	if err := os.WriteFile(deployPth, buffer, 0664); err != nil {
		interrupt.failWithMessage("Export outputs: failed to write buffer to %s: %s", deployPth, err)
	}
	return deployPth
}
