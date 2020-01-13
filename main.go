package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-steputils/stepconf"
)

type config struct {
	AdditionalParams string `env:"additional_params"`
	ProjectLocation  string `env:"project_location,dir"`
	TestLocation     string `env:"test_output_folder"`
}

func failf(msg string, args ...interface{}) {
	log.Errorf(msg, args...)
	os.Exit(1)
}

func main() {
	var cfg config
	if err := stepconf.Parse(&cfg); err != nil {
		failf("Issue with input: %s", err)
	}
	stepconf.Print(cfg)
	// additionalParams, err := shellquote.Split(cfg.AdditionalParams)
	// if err != nil {
	// 	failf("Failed to parse additional parameters, error: %s", err)
	// }

	command.New("export PATH=\"$PATH\":\"/usr/local/flutter/.pub-cache/bin\"").Run()
	command.New("export PATH=\"$PATH\":\"/usr/local/flutter/bin/cache/dart-sdk/bin\"").Run()

	log.Infof(os.Getenv("PATH"))

	command.New("flutter pub get").
		SetStderr(os.Stderr).
		SetDir(cfg.ProjectLocation).Run()

	command.New("flutter pub global activate junitreport").
		SetStderr(os.Stderr).
		SetDir(cfg.ProjectLocation).Run()

	//  command.New("\"").
	//  	SetStderr(os.Stderr).
	//  	SetDir(cfg.ProjectLocation).Run()

	// makeOutputFile := command.New("mkdir -R " + os.Getenv("test_output_folder") + "/").
	// 	SetStderr(os.Stderr).
	// 	SetDir(cfg.ProjectLocation)

	// fmt.Println()
	// log.Donef("$ %s", makeOutputFile.PrintableCommandArgs())
	// fmt.Println()

	// if errMkdir := makeOutputFile.Run(); errMkdir != nil {
	// 	failf("Running command failed, error: %s", errMkdir)
	// }

	fmt.Println()
	log.Infof("Running test")

	outputFile, fCreationError := os.Create("test_results.jsonl")
	if fCreationError != nil {
		failf("Output ffile creation failed, error: %s", fCreationError)
	}
	writer := bufio.NewWriter(outputFile)

	testCmd := command.New("flutter", append([]string{"test"}, "--machine")...).
		SetStdout(writer).
		SetStderr(os.Stderr).
		SetDir(cfg.ProjectLocation)

	fmt.Println()
	log.Donef("$ %s", testCmd.PrintableCommandArgs())
	fmt.Println()

	if err := testCmd.Run(); err != nil {
		failf("Running command failed, error: %s", err)
	}

	/*convertReportCommand := command.New("tojunit -i test_results.jsonl -o export.xml")
	//+
	//cfg.TestLocation + "/TEST-report.xml")
	fmt.Println()
	log.Donef("$ %s", convertReportCommand.PrintableCommandArgs())
	fmt.Println()

	if errReport := convertReportCommand.Run(); errReport != nil {
		failf("Running command failed, error: %s", errReport)
	}*/

	//addTestInfoFile := command.New("echo '{\"test-name\":\"tests-batch-1\"}' >> \"$test_run_dir/test-info.json\"")

	// fmt.Println()
	// log.Donef("$ %s", addTestInfoFile.PrintableCommandArgs())
	// fmt.Println()

	// if errorInfoFile := addTestInfoFile.Run(); errorInfoFile != nil {
	// 	failf("Running command failed, error: %s", errorInfoFile)
	// }

}
