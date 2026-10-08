package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/testattachment"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The shape tojunit writes: the class name is the test file's absolute path without "_test.dart",
// and the name is the full test name, group included.
const flutterJUnitXML = `<testsuites>
  <testsuite name=".Users.vagrant.git.test.login_flow">
    <testcase classname=".Users.vagrant.git.test.login_flow" name="Login shows an error for a wrong password"/>
    <testcase classname=".Users.vagrant.git.test.login_flow" name="Login accepts a valid password"/>
  </testsuite>
</testsuites>`

const flutterAttachmentName = ".Users.vagrant.git.test.login_flow__Login shows an error for a wrong password__1.png"

func Test_exportConventionAttachments(t *testing.T) {
	projectDir := t.TempDir()
	junitPath := filepath.Join(projectDir, testResultFileName)
	writeTestFile(t, junitPath, flutterJUnitXML)
	writeTestFile(t, filepath.Join(projectDir, "test", "screenshots", flutterAttachmentName), "screenshot")
	reportDir := t.TempDir()

	exportConventionAttachments(log.NewLogger(), newTestCollector(), projectDir, t.TempDir(), junitPath, reportDir)

	assertTestFile(t, filepath.Join(reportDir, flutterAttachmentName), "screenshot")
}

func Test_exportConventionAttachments_invalidXML(t *testing.T) {
	projectDir := t.TempDir()
	junitPath := filepath.Join(projectDir, testResultFileName)
	writeTestFile(t, junitPath, "not xml")
	writeTestFile(t, filepath.Join(projectDir, flutterAttachmentName), "screenshot")
	reportDir := t.TempDir()
	var logs bytes.Buffer

	exportConventionAttachments(log.NewLogger(log.WithOutput(&logs)), newTestCollector(), projectDir, t.TempDir(), junitPath, reportDir)

	assert.Contains(t, logs.String(), "Failed to read test cases")
	assert.NoFileExists(t, filepath.Join(reportDir, flutterAttachmentName))
}

func Test_exportConventionAttachments_missingDeployDir(t *testing.T) {
	projectDir := t.TempDir()
	junitPath := filepath.Join(projectDir, testResultFileName)
	writeTestFile(t, junitPath, flutterJUnitXML)
	writeTestFile(t, filepath.Join(projectDir, flutterAttachmentName), "screenshot")
	reportDir := t.TempDir()
	var logs bytes.Buffer

	exportConventionAttachments(log.NewLogger(log.WithOutput(&logs)), newTestCollector(), projectDir, "", junitPath, reportDir)

	assert.Contains(t, logs.String(), "BITRISE_TEST_DEPLOY_DIR is not set")
	assertTestFile(t, filepath.Join(reportDir, flutterAttachmentName), "screenshot")
}

func newTestCollector() testattachment.Collector {
	return testattachment.NewCollector(command.NewFactory(env.NewRepository()), fileutil.NewFileManager())
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func assertTestFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if assert.NoError(t, err) {
		assert.Equal(t, want, string(got))
	}
}
