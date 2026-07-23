package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
)

type commandBuilder interface {
	supportsFileReporter() bool
	buildTestCmd(generateCoverage bool, fileReporterPath string, additionalParams []string) commandWrapper
	buildJunitCmd(cfg config, jsonReportPath string) commandWrapper
	buildLegacyTestCmd(generateCoverage bool, additionalParams []string) commandWrapper
	buildLegacyJunitCmd(cfg config) commandWrapper
}

type realCommandBuilder struct {
	interrupt interrupt
}

func (r realCommandBuilder) ensureToJunitAvailable(cfg config) {
	// Always (re)activate junitreport to ensure the compiled binary and lock file
	// are valid for the current Dart SDK version. A pre-installed tojunit binary
	// compiled with a different Dart version causes "Can't load Kernel binary"
	// errors at runtime; a stale lock file (e.g. a transitive dep was bumped in
	// the pub cache) triggers exit-65 from dart pub. Both are cured by reactivating.
	log.Infof("Activating `tojunit`...")
	junitInstallCmd := command.New("dart", []string{"pub", "global", "activate", "--overwrite", "junitreport"}...).
		SetStdout(os.Stdout).
		SetStderr(os.Stderr).
		SetDir(cfg.ProjectLocation)

	fmt.Println()
	log.Donef(fmt.Sprintf("$ %s", junitInstallCmd.PrintableCommandArgs()))
	fmt.Println()

	if err := junitInstallCmd.Run(); err != nil {
		if errorutil.IsExitStatusError(err) {
			r.interrupt.failWithMessage("Install dependencies: command `tojunit` failed to install: %s", err)
		}
		r.interrupt.failWithMessage("Install dependencies: failed to run command `tojunit`: %s", err)
	}
}

// supportsFileReporter reports whether the installed `flutter test` accepts the
// --file-reporter flag. It was added in Flutter 3.10; on older versions the step
// falls back to piping `flutter test --machine` into tojunit. We probe the actual
// `flutter test --help` output rather than parsing versions so custom channels and
// forks are handled correctly. On any error we assume it is unsupported.
func (r realCommandBuilder) supportsFileReporter() bool {
	out, err := exec.Command("flutter", "test", "--help").CombinedOutput()
	if err != nil {
		log.Warnf("Could not determine whether `flutter test` supports --file-reporter (%s); falling back to --machine.", err)
		return false
	}
	return strings.Contains(string(out), "--file-reporter")
}

// buildTestCmd builds the modern test command. It writes the machine-readable JSON to
// a file via --file-reporter (instead of --machine) so that stdout keeps the normal,
// human-readable reporter output. With --reporter unset, Flutter auto-selects a
// CI-friendly reporter (expanded on non-TTY, github on GitHub Actions); users can
// still override it through additionalParams.
func (r realCommandBuilder) buildTestCmd(generateCoverage bool, fileReporterPath string, additionalParams []string) commandWrapper {
	params := []string{"test", "--file-reporter=json:" + fileReporterPath}
	if generateCoverage {
		params = append(params, "--coverage")
	}
	params = append(params, additionalParams...)

	return realCommandWrapper{cmd: exec.Command("flutter", params...)}
}

// buildJunitCmd converts the flutter test JSON report at jsonReportPath into JUnit XML.
func (r realCommandBuilder) buildJunitCmd(cfg config, jsonReportPath string) commandWrapper {
	r.ensureToJunitAvailable(cfg)
	// Use "dart pub global run" instead of invoking tojunit by name so that the
	// executable is found even when $HOME/.pub-cache/bin is not on $PATH (Linux).
	// tojunit reads the JSON from --input and writes the JUnit XML to --output.
	return realCommandWrapper{cmd: exec.Command("dart", []string{"pub", "global", "run", "junitreport:tojunit", "--input", jsonReportPath, "--output", testResultFileName}...)}
}

// buildLegacyTestCmd builds the test command for Flutter versions without --file-reporter
// (< 3.10): `flutter test --machine`, whose JSON stdout is piped straight into tojunit.
func (r realCommandBuilder) buildLegacyTestCmd(generateCoverage bool, additionalParams []string) commandWrapper {
	params := []string{"test", "--machine"}
	if generateCoverage {
		params = append(params, "--coverage")
	}
	params = append(params, additionalParams...)

	return realCommandWrapper{cmd: exec.Command("flutter", params...)}
}

// buildLegacyJunitCmd converts the flutter test JSON read from stdin into JUnit XML.
func (r realCommandBuilder) buildLegacyJunitCmd(cfg config) commandWrapper {
	r.ensureToJunitAvailable(cfg)
	// Use "dart pub global run" instead of invoking tojunit by name so that the
	// executable is found even when $HOME/.pub-cache/bin is not on $PATH (Linux).
	return realCommandWrapper{cmd: exec.Command("dart", []string{"pub", "global", "run", "junitreport:tojunit", "--output", testResultFileName}...)}
}
