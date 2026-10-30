package main

import (
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-steputils/v2/export"
	"github.com/bitrise-io/go-steputils/v2/stepconf"
	"github.com/bitrise-io/go-steputils/v2/testattachment"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-steplib/bitrise-step-flutter-test/testselection"
)

type config struct {
	AdditionalParams          string `env:"additional_params"`
	TestsPathPattern          string `env:"tests_path_pattern"`
	ProjectLocation           string `env:"project_location,dir"`
	TestResultsDir            string `env:"bitrise_test_result_dir,dir"`
	GenerateCodeCoverageFiles bool   `env:"generate_code_coverage_files,opt[yes,no]"`
	TestSelection             string `env:"test_selection"`
	OnEmptySelection          string `env:"on_empty_selection"`
}

var logger = log.NewLogger()
var envRepo = env.NewRepository()
var cmdFactory = command.NewFactory(envRepo)
var fileManager = fileutil.NewFileManager()
var ir interrupt = realInterrupt{logger: logger}
var parser configParser = realConfigParser{interrupt: ir, logger: logger, envRepo: envRepo}
var builder commandBuilder = realCommandBuilder{interrupt: ir, logger: logger, cmdFactory: cmdFactory}
var test testExecutor = realTestExecutor{
	interrupt:      ir,
	logger:         logger,
	commandBuilder: builder,
	testExporter: realTestExporter{
		interrupt:           ir,
		logger:              logger,
		outputExporter:      export.NewDefaultExporter(cmdFactory),
		fileManager:         fileManager,
		attachmentCollector: testattachment.NewCollector(cmdFactory, fileManager),
		testDeployDir:       envRepo.Get("BITRISE_TEST_DEPLOY_DIR"),
	},
}

func main() {
	cfg := parser.parseConfig()

	stepconf.Print(cfg)

	additionalParams := parser.parseAdditionalParams(cfg.AdditionalParams)

	sel, decision, err := processTestSelection(cfg.TestSelection, cfg.OnEmptySelection)
	if err != nil {
		ir.failWithMessage("Process config: %s", err)
		return
	}
	if decision.Action == testselection.Skip {
		fmt.Println()
		logger.Infof("Skipping the test run: %s", decision.Reason)
		return
	}
	packageDirs := []string{cfg.ProjectLocation}
	switch {
	case decision.Action == testselection.RunSelected:
		packageDirs = selectedPackageDirs(decision.Tests)
	case sel != nil && len(sel.Selected("flutter")) > 0:
		// A full or shadow run still lists every candidate package, so a monorepo runs
		// all of them rather than only project_location.
		packageDirs = selectedPackageDirs(sel.Selected("flutter"))
	}

	failed := false
	for _, dir := range packageDirs {
		pkgCfg := cfg
		pkgCfg.ProjectLocation = dir
		if len(packageDirs) > 1 {
			testNameSuffix = filepath.Base(dir)
		}

		testPaths := parser.expandTestsPathPattern(pkgCfg.ProjectLocation, cfg.TestsPathPattern)
		params := append(append([]string{}, additionalParams...), testPaths...)

		fmt.Println()
		logger.Infof("Running test in %s", dir)

		outputBuffer, testErr := test.executeTest(pkgCfg, params)
		test.exportTestResults(pkgCfg, outputBuffer)
		if testErr {
			failed = true
		}
	}

	if failed {
		ir.fail()
	}
}

// processTestSelection reads the Test Selection step's output for Flutter packages.
func processTestSelection(path, onEmpty string) (*testselection.Selection, testselection.Decision, error) {
	sel, err := testselection.Load(path)
	if err != nil {
		return nil, testselection.Decision{}, err
	}
	decision, err := testselection.Decide(sel, onEmpty, "flutter")
	if err != nil {
		return nil, testselection.Decision{}, err
	}
	if sel != nil {
		fmt.Println()
		logger.Infof("Test selection: %s", decision.Reason)
	}
	return sel, decision, nil
}

// selectedPackageDirs resolves the selected packages' directories. The Test Selection
// step writes them relative to the repository root.
func selectedPackageDirs(tests []testselection.Test) []string {
	root := envRepo.Get("BITRISE_SOURCE_DIR")
	var dirs []string
	for _, t := range tests {
		dir := t.Identifiers.PackageDir
		if dir == "" {
			continue
		}
		if !filepath.IsAbs(dir) && root != "" {
			dir = filepath.Join(root, dir)
		}
		logger.Printf("- %s (%s)", dir, t.Reason)
		dirs = append(dirs, dir)
	}
	return dirs
}
