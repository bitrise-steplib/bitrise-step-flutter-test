package main

import (
	"fmt"

	"github.com/bitrise-io/go-steputils/v2/export"
	"github.com/bitrise-io/go-steputils/v2/stepconf"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
)

type config struct {
	AdditionalParams          string `env:"additional_params"`
	TestsPathPattern          string `env:"tests_path_pattern"`
	ProjectLocation           string `env:"project_location,dir"`
	TestResultsDir            string `env:"bitrise_test_result_dir,dir"`
	GenerateCodeCoverageFiles bool   `env:"generate_code_coverage_files,opt[yes,no]"`
}

var logger = log.NewLogger()
var envRepo = env.NewRepository()
var cmdFactory = command.NewFactory(envRepo)
var ir interrupt = realInterrupt{logger: logger}
var parser configParser = realConfigParser{interrupt: ir, logger: logger, envRepo: envRepo}
var builder commandBuilder = realCommandBuilder{interrupt: ir, logger: logger, cmdFactory: cmdFactory}
var test testExecutor = realTestExecutor{
	interrupt:      ir,
	logger:         logger,
	commandBuilder: builder,
	testExporter: realTestExporter{
		interrupt:      ir,
		logger:         logger,
		outputExporter: export.NewDefaultExporter(cmdFactory),
		fileManager:    fileutil.NewFileManager(),
	},
}

func main() {
	cfg := parser.parseConfig()

	stepconf.Print(cfg)

	additionalParams := parser.parseAdditionalParams(cfg.AdditionalParams)

	testPaths := parser.expandTestsPathPattern(cfg.ProjectLocation, cfg.TestsPathPattern)

	additionalParams = append(additionalParams, testPaths...)

	fmt.Println()
	logger.Infof("Running test")

	outputBuffer, testErr := test.executeTest(cfg, additionalParams)
	test.exportTestResults(cfg, outputBuffer)

	if testErr {
		ir.fail()
	}
}
