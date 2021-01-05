package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/testresultexport/testresultexport"
	"github.com/bitrise-tools/go-steputils/stepconf"
	"github.com/bitrise-tools/go-steputils/tools"
	"github.com/kballard/go-shellquote"
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

func failf(msg string, args ...interface{}) {
	log.Errorf(msg, args...)
	os.Exit(1)
}

func copyToDeployDir(logPth string, logFileName string) string {
	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
	if deployDir == "" {
		failf("no BITRISE_DEPLOY_DIR found")
	}
	deployPth := filepath.Join(deployDir, logFileName)

	if err := command.CopyFile(logPth, deployPth); err != nil {
		failf("failed to copy `%s` info file from (%s) to (%s), error: %s", logFileName, logPth, deployPth, err)
	}
	return deployPth
}

func copyBufferToDeployDir(buffer []byte, logFileName string) string {
	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
	if deployDir == "" {
		failf("no BITRISE_DEPLOY_DIR found")
	}
	deployPth := filepath.Join(deployDir, logFileName)

	if err := ioutil.WriteFile(deployPth, buffer, 0664); err != nil {
		failf("failed to write buffer to (%s), error: %s", deployPth, err)
	}
	return deployPth
}

func parseConfig() config {
	var cfg config
	if err := stepconf.Parse(&cfg); err != nil {
		failf("Issue with input: %s", err)
	}
	return cfg
}

func parseAdditionalParams(cfg config) []string {
	additionalParams, err := shellquote.Split(cfg.AdditionalParams)
	if err != nil {
		failf("Failed to parse additional parameters, error: %s", err)
	}
	return additionalParams
}

func ensureToJunitAvailable(cfg config) {
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
}

func executeTest(cfg config, additionalParams []string) (bytes.Buffer, bool) {
	var jsonBuffer bytes.Buffer
	pr, pw := io.Pipe()
	testCmdWriter := io.MultiWriter(pw, &jsonBuffer)

	ensureToJunitAvailable(cfg)

	testCmd := exec.Command("flutter", append([]string{"test", "--machine"}, additionalParams...)...)
	junitCmd := exec.Command("tojunit", append([]string{"--output", testResultFileName})...)

	testExecutionFailed := false

	testCmdModel := command.NewWithCmd(testCmd).
		SetStdout(testCmdWriter).
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
		log.Errorf("Completing test command failed, error: %s", err)
		testExecutionFailed = true
	}

	if err := pw.Close(); err != nil {
		failf("Closing pipe failed, error: %s", err)
	}

	if err := junitCmd.Wait(); err != nil {
		failf("Completing conversion command failed, error: %s", err)
	}
	return jsonBuffer, testExecutionFailed
}

func exportTestResults(cfg config, jsonBuffer bytes.Buffer) {
	testResultDeployPath := copyBufferToDeployDir(jsonBuffer.Bytes(), testResultJSONFileName)
	if err := tools.ExportEnvironmentWithEnvman("BITRISE_FLUTTER_TESTRESULT_PATH", testResultDeployPath); err != nil {
		failf("Failed to export: BITRISE_FLUTTER_TESTRESULT_PATH, error: %s", err)
	}

	exporter := testresultexport.NewExporter(cfg.TestResultsDir)
	if err := exporter.ExportTest(testName, testResultFileName); err != nil {
		failf("Failed to export test result: %s", err)
	}
}

func executeCoverage(additionalParams []string) bool {
	coverageCmdModel := command.New("flutter", append([]string{"test", "--coverage"}, additionalParams...)...)

	fmt.Println()
	log.Infof("Rerunning test command to generate coverage data")
	fmt.Println()
	log.Donef("$ %s", coverageCmdModel.PrintableCommandArgs())
	fmt.Println()

	if err := coverageCmdModel.Run(); err != nil {
		log.Errorf("Completing coverage command failed, error: %s", err)
		return true
	}
	return false
}

func exportCoverage() {
	coverageDeployPath := copyToDeployDir(coveragePath, coverageFileName)
	if err := tools.ExportEnvironmentWithEnvman("BITRISE_FLUTTER_COVERAGE_PATH", coverageDeployPath); err != nil {
		failf("Failed to export: BITRISE_FLUTTER_COVERAGE_PATH, error: %s", err)
	}
}

func main() {
	cfg := parseConfig()

	stepconf.Print(cfg)

	additionalParams := parseAdditionalParams(cfg)

	fmt.Println()
	log.Infof("Running test")

	jsonBuffer, testExecutionFailed := executeTest(cfg, additionalParams)
	exportTestResults(cfg, jsonBuffer)

	if cfg.GenerateCodeCoverageFiles {
		testExecutionFailed = executeCoverage(additionalParams)
		exportCoverage()
	}

	log.Infof("test results exported in junit format successfully")

	if testExecutionFailed {
		os.Exit(1)
	}
}
