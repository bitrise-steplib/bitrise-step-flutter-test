package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
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
	const testName = "Flutter test results"
	const testResultFileName = "./flutter_junit_test_results.xml"
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

	pr, pw := io.Pipe()

	if _, err := exec.LookPath("tojunit"); err != nil {
		log.Infof("Command `tojunit` not found, installing...")
		junitInstallCmd := command.New("flutter", append([]string{"pub", "global", "activate", "junitreport"})...).
			SetStdout(os.Stdout).
			SetStderr(os.Stderr).
			SetDir(cfg.ProjectLocation)

		fmt.Println()
		log.Donef(fmt.Sprintf("$ %s", junitInstallCmd.PrintableCommandArgs()))
		fmt.Println()

		if err := junitInstallCmd.Run(); err != nil {
			if errorutil.IsExitStatusError(err) {
				failf("Command `tojunit` failed to install, error: %s", err)
			}
			failf("Failed to run command `tojunit`, %s", err)
		}
	}

	testCmd := exec.Command("flutter", append([]string{"test", "--machine"}, additionalParams...)...)
	junitCmd := exec.Command("tojunit", append([]string{"--output", testResultFileName})...)

	testCmdModel := command.NewWithCmd(testCmd).
		SetStdout(pw).
		SetStderr(os.Stderr).
		SetDir(cfg.ProjectLocation)

	junitCmdModel := command.NewWithCmd(junitCmd).
		SetStdin(pr).
		SetStdout(os.Stdout).
		SetStderr(os.Stderr).
		SetDir(cfg.ProjectLocation)

	fmt.Println()
	log.Donef("$ %s | %s", testCmdModel.PrintableCommandArgs(), junitCmdModel.PrintableCommandArgs())
	fmt.Println()

	if err := testCmd.Start(); err != nil {
		failf("Running command failed, error: %s", err)
	}

	if err := junitCmd.Start(); err != nil {
		failf("Converting test results to junit format failed, error: %s", err)
	}

	if err := testCmd.Wait(); err != nil {
		failf("Completing test command failed, error: %s", err)
	}

	if err := pw.Close(); err != nil {
		failf("Closing pipe failed, error: %s", err)
	}

	if err := junitCmd.Wait(); err != nil {
		failf("Completing conversion command failed, error: %s", err)
	}

	exporter := testresultexport.NewExporter(cfg.TestResultsDir)
	if err := exporter.ExportTest(testName, testResultFileName); err != nil {
		failf("Failed to export test result: %s", err)
	}

	log.Infof("test results exported in junit format successfully")
}
