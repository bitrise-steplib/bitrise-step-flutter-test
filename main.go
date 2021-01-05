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

// region interrupt

type interrupt interface {
	failWithMessage(msg string, args ...interface{})
	fail()
}

type realInterrupt struct{}

func (r realInterrupt) failWithMessage(msg string, args ...interface{}) {
	log.Errorf(msg, args...)
	r.fail()
}

func (r realInterrupt) fail() {
	os.Exit(1)
}

// endregion

// region configParser

type configParser interface {
	parseConfig() config
	parseAdditionalParams(cfg config) []string
}

type realConfigParser struct {
	interrupt interrupt
}

func (r realConfigParser) parseConfig() config {
	var cfg config
	if err := stepconf.Parse(&cfg); err != nil {
		r.interrupt.failWithMessage("Issue with input: %s", err)
	}
	return cfg
}

func (r realConfigParser) parseAdditionalParams(cfg config) []string {
	additionalParams, err := shellquote.Split(cfg.AdditionalParams)
	if err != nil {
		r.interrupt.failWithMessage("Failed to parse additional parameters, error: %s", err)
	}
	return additionalParams
}

// endregion

// region commandWrapper

type commandWrapper interface {
	start() error
	wait() error
	unwrap() *exec.Cmd
}

type realCommandWrapper struct {
	cmd *exec.Cmd
}

func (w realCommandWrapper) start() error {
	return w.cmd.Start()
}

func (w realCommandWrapper) wait() error {
	return w.cmd.Wait()
}

func (w realCommandWrapper) unwrap() *exec.Cmd {
	return w.cmd
}

// endregion

// region modelWrapper

type modelWrapper interface {
	PrintableCommandArgs() string
	Run() error
}

type realModelWrapper struct {
	model *command.Model
}

func (r realModelWrapper) Run() error {
	return r.model.Run()
}

func (r realModelWrapper) PrintableCommandArgs() string {
	return r.model.PrintableCommandArgs()
}

// endregion

// region commandBuilder

type commandBuilder interface {
	buildTestCmd(additionalParams []string) commandWrapper
	buildJunitCmd(cfg config) commandWrapper
	buildCoverageCmd(additionalParams []string) modelWrapper
}

type realCommandBuilder struct {
	interrupt interrupt
}

func (r realCommandBuilder) ensureToJunitAvailable(cfg config) {
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
				r.interrupt.failWithMessage("Command `tojunit` failed to install, error: %s", err)
			}
			r.interrupt.failWithMessage("Failed to run command `tojunit`, %s", err)
		}
	}
}

func (r realCommandBuilder) buildTestCmd(additionalParams []string) commandWrapper {
	return realCommandWrapper{cmd: exec.Command("flutter", append([]string{"test", "--machine"}, additionalParams...)...)}
}

func (r realCommandBuilder) buildJunitCmd(cfg config) commandWrapper {
	r.ensureToJunitAvailable(cfg)
	return realCommandWrapper{cmd: exec.Command("tojunit", append([]string{"--output", testResultFileName})...)}
}

func (r realCommandBuilder) buildCoverageCmd(additionalParams []string) modelWrapper {
	return realModelWrapper{model: command.New("flutter", append([]string{"test", "--coverage"}, additionalParams...)...)}
}

// endregion

// region testExecutor

type testExecutor interface {
	executeTest(cfg config, additionalParams []string) (bytes.Buffer, bool)
	exportTestResults(cfg config, jsonBuffer bytes.Buffer)
}

type realTestExecutor struct {
	interrupt      interrupt
	commandBuilder commandBuilder
}

func (r realTestExecutor) executeTest(cfg config, additionalParams []string) (bytes.Buffer, bool) {
	var jsonBuffer bytes.Buffer
	pr, pw := io.Pipe()
	testCmdWriter := io.MultiWriter(pw, &jsonBuffer)

	testCmd := r.commandBuilder.buildTestCmd(additionalParams)
	junitCmd := r.commandBuilder.buildJunitCmd(cfg)

	testExecutionFailed := false

	testCmdModel := command.NewWithCmd(testCmd.unwrap()).
		SetStdout(testCmdWriter).
		SetStderr(os.Stderr).
		SetDir(cfg.ProjectLocation)

	junitCmdModel := command.NewWithCmd(junitCmd.unwrap()).
		SetStdin(pr).
		SetStdout(os.Stdout).
		SetStderr(os.Stderr).
		SetDir(cfg.ProjectLocation)

	fmt.Println()
	log.Donef("$ %s | %s", testCmdModel.PrintableCommandArgs(), junitCmdModel.PrintableCommandArgs())
	fmt.Println()

	if err := testCmd.start(); err != nil {
		r.interrupt.failWithMessage("Running command failed, error: %s", err)
	}

	if err := junitCmd.start(); err != nil {
		r.interrupt.failWithMessage("Converting test results to junit format failed, error: %s", err)
	}

	if err := testCmd.wait(); err != nil {
		log.Errorf("Completing test command failed, error: %s", err)
		testExecutionFailed = true
	}

	if err := pw.Close(); err != nil {
		r.interrupt.failWithMessage("Closing pipe failed, error: %s", err)
	}

	if err := junitCmd.wait(); err != nil {
		r.interrupt.failWithMessage("Completing conversion command failed, error: %s", err)
	}
	return jsonBuffer, testExecutionFailed
}

func (r realTestExecutor) exportTestResults(cfg config, jsonBuffer bytes.Buffer) {
	testResultDeployPath := copyBufferToDeployDir(jsonBuffer.Bytes(), testResultJSONFileName, r.interrupt)
	if err := tools.ExportEnvironmentWithEnvman("BITRISE_FLUTTER_TESTRESULT_PATH", testResultDeployPath); err != nil {
		r.interrupt.failWithMessage("Failed to export: BITRISE_FLUTTER_TESTRESULT_PATH, error: %s", err)
	}

	exporter := testresultexport.NewExporter(cfg.TestResultsDir)
	if err := exporter.ExportTest(testName, testResultFileName); err != nil {
		r.interrupt.failWithMessage("Failed to export test result: %s", err)
	}
}

// endregion

// region coverageExecutor

type coverageExecutor interface {
	executeCoverage(additionalParams []string) bool
	exportCoverage()
}

type realCoverageExecutor struct {
	interrupt      interrupt
	commandBuilder commandBuilder
}

func (r realCoverageExecutor) executeCoverage(additionalParams []string) bool {
	coverageCmdModel := r.commandBuilder.buildCoverageCmd(additionalParams)

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

func (r realCoverageExecutor) exportCoverage() {
	coverageDeployPath := copyToDeployDir(coveragePath, coverageFileName, r.interrupt)
	if err := tools.ExportEnvironmentWithEnvman("BITRISE_FLUTTER_COVERAGE_PATH", coverageDeployPath); err != nil {
		r.interrupt.failWithMessage("Failed to export: BITRISE_FLUTTER_COVERAGE_PATH, error: %s", err)
	}
}

// endregion

func copyToDeployDir(logPth string, logFileName string, interrupt interrupt) string {
	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
	if deployDir == "" {
		interrupt.failWithMessage("no BITRISE_DEPLOY_DIR found")
	}
	deployPth := filepath.Join(deployDir, logFileName)

	if err := command.CopyFile(logPth, deployPth); err != nil {
		interrupt.failWithMessage("failed to copy `%s` info file from (%s) to (%s), error: %s", logFileName, logPth, deployPth, err)
	}
	return deployPth
}

func copyBufferToDeployDir(buffer []byte, logFileName string, interrupt interrupt) string {
	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")
	if deployDir == "" {
		interrupt.failWithMessage("no BITRISE_DEPLOY_DIR found")
	}
	deployPth := filepath.Join(deployDir, logFileName)

	if err := ioutil.WriteFile(deployPth, buffer, 0664); err != nil {
		interrupt.failWithMessage("failed to write buffer to (%s), error: %s", deployPth, err)
	}
	return deployPth
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
