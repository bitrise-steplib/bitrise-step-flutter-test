package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
)

type commandBuilder interface {
	buildTestCmd(generateCoverage bool, additionalParams []string) commandWrapper
	buildJunitCmd(cfg config) commandWrapper
}

type realCommandBuilder struct {
	interrupt interrupt
}

func (r realCommandBuilder) ensureToJunitAvailable(cfg config) {
	if _, err := exec.LookPath("tojunit"); err != nil {
		log.Infof("Command `tojunit` not found, installing...")
		junitInstallCmd := command.New("flutter", []string{"pub", "global", "activate", "junitreport"}...).
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
}

func (r realCommandBuilder) buildTestCmd(generateCoverage bool, additionalParams []string) commandWrapper {
	params := []string{"test", "--machine"}
	if generateCoverage {
		params = append(params, "--coverage")
	}
	params = append(params, additionalParams...)

	return realCommandWrapper{cmd: exec.Command("flutter", params...)}
}

func (r realCommandBuilder) buildJunitCmd(cfg config) commandWrapper {
	r.ensureToJunitAvailable(cfg)
	return realCommandWrapper{cmd: exec.Command("tojunit", []string{"--output", testResultFileName}...)}
}
