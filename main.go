package main

import (
	"fmt"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-steputils/stepconf"
)

const (
	coverageFileName       = "flutter_coverage_lcov.info"
	coveragePath           = "./coverage/lcov.info"
	testName               = "Flutter test results"
	testResultFileName     = "./flutter_junit_test_results.xml"
	testResultJSONFileName = "flutter_json_test_results.json"
)

type config struct {
	AdditionalParams          string `env:"additional_params"`
	ProjectLocation           string `env:"project_location,dir"`
	TestResultsDir            string `env:"bitrise_test_result_dir,dir"`
	GenerateCodeCoverageFiles bool   `env:"generate_code_coverage_files,opt[yes,no]"`
}

var ir interrupt = realInterrupt{}
var parser configParser = realConfigParser{interrupt: ir}
var builder commandBuilder = realCommandBuilder{interrupt: ir}
var test testExecutor = realTestExecutor{interrupt: ir, commandBuilder: builder}
var coverage coverageExecutor = realCoverageExecutor{interrupt: ir, commandBuilder: builder}

func main() {
	cfg := parser.parseConfig()

	stepconf.Print(cfg)

	additionalParams := parser.parseAdditionalParams(cfg)

	fmt.Println()
	log.Infof("Running test")

	jsonBuffer, testExecutionFailed := test.executeTest(cfg, additionalParams)
	test.exportTestResults(cfg, jsonBuffer)

	var coverageExecutionFailed bool
	if cfg.GenerateCodeCoverageFiles {
		coverageExecutionFailed = coverage.executeCoverage(additionalParams)
		coverage.exportCoverage()
	}

	log.Infof("test results exported in junit format successfully")

	if testExecutionFailed || coverageExecutionFailed {
		ir.fail()
	}
}
