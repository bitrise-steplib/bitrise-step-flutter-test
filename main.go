package main

import (
	"fmt"
	"os"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/testresultexport/testresultexport"
	"github.com/bitrise-tools/go-steputils/stepconf"
	shellquote "github.com/kballard/go-shellquote"
)

type config struct {
	AdditionalParams string `env:"additional_params"`
	ProjectLocation  string `env:"project_location,dir"`
	TestResultsDir   string `env:"bitrise_test_result_dir,dir"`
}

func failf(msg string, args ...interface{}) {
	log.Errorf(msg, args...)
	os.Exit(1)
}

func main() {
	const TestName = "Flutter test results"
	const TestResultFileName = "flutter_junit_test_results.xml"
	var cfg config
	if err := stepconf.Parse(&cfg); err != nil {
		failf("Issue with input: %s", err)
	}
	stepconf.Print(cfg)

	additionalParams, err := shellquote.Split(cfg.AdditionalParams)
	if err != nil {
		failf("Failed to parse additional parameters, error: %s", err)
	}

	fmt.Println()
	log.Infof("Running test")

	testCmd := command.New("flutter", append([]string{"test", "--machine | tojunit --output", TestResultFileName}, additionalParams...)...).
		SetStdout(os.Stdout).
		SetStderr(os.Stderr).
		SetDir(cfg.ProjectLocation)

	fmt.Println()
	log.Donef("$ %s", testCmd.PrintableCommandArgs())
	fmt.Println()

	output, err := testCmd.RunAndReturnTrimmedOutput()
	if err != nil {
		failf("Running command failed, error: %s", err)
	}

	exporter := testresultexport.NewExporter(cfg.TestResultsDir)
	if err := exporter.ExportTest(TestName, output); err != nil {
		failf("Failed to export test result: %s", err)
	}
}
