package main

import (
	"fmt"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
	"os"
	"os/exec"
)

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
