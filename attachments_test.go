package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The shape tojunit writes: a test's prints go to its system-out, its errors to its error.
func junitXML(errorOutput, systemOut string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite errors="1" failures="0" tests="2" skipped="0" name=".Users.vagrant.git.test.login" timestamp="2026-10-08T09:00:00">
    <testcase classname=".Users.vagrant.git.test.login" name="Login shows an error" time="0.1">
      <error message="1 error, see stacktrace for details">` + errorOutput + `</error>
      <system-out>` + systemOut + `</system-out>
    </testcase>
    <testcase classname=".Users.vagrant.git.test.login" name="Login accepts a valid password" time="0.1"/>
  </testsuite>
</testsuites>
`
}

func Test_exportTestAttachments_marker(t *testing.T) {
	projectDir, reportDir := setUpReport(t, junitXML("", "[[ATTACHMENT|build/shots/error.png]]"))
	writeTestFile(t, filepath.Join(projectDir, "build", "shots", "error.png"), "screenshot")

	exportTestAttachments(log.NewLogger(), fileutil.NewFileManager(), projectDir, reportDir)

	assertTestFile(t, filepath.Join(reportDir, "error.png"), "screenshot")
	assert.Equal(t, []string{"error.png"}, attachmentsOf(t, reportDir, "Login shows an error"))
	assert.Empty(t, attachmentsOf(t, reportDir, "Login accepts a valid password"))
}

func Test_exportTestAttachments_goldenFailure(t *testing.T) {
	projectDir := t.TempDir()
	failuresDir := filepath.Join(projectDir, "test", "failures")
	// The wrapped shape flutter_test prints a golden failure in. A size mismatch writes only two images.
	output := `══╡ EXCEPTION CAUGHT BY FLUTTER TEST FRAMEWORK ╞═══
The following assertion was thrown while running async test code:
Golden "goldens/login.png": Pixel test failed, image sizes do not match.
Master Image: 800 X 600
Test Image: 400 X 300
Failure feedback can be found at
` + failuresDir + `

When the exception was thrown, this was the stack:`
	reportDir := writeReport(t, junitXML("Test failed. See exception logs above.", output))
	writeTestFile(t, filepath.Join(failuresDir, "login_masterImage.png"), "master")
	writeTestFile(t, filepath.Join(failuresDir, "login_testImage.png"), "test")
	var logs bytes.Buffer

	exportTestAttachments(log.NewLogger(log.WithOutput(&logs)), fileutil.NewFileManager(), projectDir, reportDir)

	assert.Equal(t, []string{"login_masterImage.png", "login_testImage.png"}, attachmentsOf(t, reportDir, "Login shows an error"))
	assert.NotContains(t, logs.String(), "Skipping")
}

func Test_exportTestAttachments_goldenFailureInError(t *testing.T) {
	projectDir := t.TempDir()
	failuresDir := filepath.Join(projectDir, "test", "failures")
	output := `Golden "goldens/login.png": Pixel test failed, 1.00%, 4800px diff detected.
Failure feedback can be found at ` + failuresDir
	reportDir := writeReport(t, junitXML(output, ""))
	for _, suffix := range []string{"masterImage", "testImage", "isolatedDiff", "maskedDiff"} {
		writeTestFile(t, filepath.Join(failuresDir, "login_"+suffix+".png"), suffix)
	}

	exportTestAttachments(log.NewLogger(), fileutil.NewFileManager(), projectDir, reportDir)

	assert.Equal(t, []string{"login_masterImage.png", "login_testImage.png", "login_isolatedDiff.png", "login_maskedDiff.png"}, attachmentsOf(t, reportDir, "Login shows an error"))
}

func Test_exportTestAttachments_sameFileNameTwice(t *testing.T) {
	projectDir, reportDir := setUpReport(t, junitXML("", "[[ATTACHMENT|a/screen.png]]\n[[ATTACHMENT|b/screen.png]]"))
	writeTestFile(t, filepath.Join(projectDir, "a", "screen.png"), "a")
	writeTestFile(t, filepath.Join(projectDir, "b", "screen.png"), "b")

	exportTestAttachments(log.NewLogger(), fileutil.NewFileManager(), projectDir, reportDir)

	assert.Equal(t, []string{"screen.png", "screen-2.png"}, attachmentsOf(t, reportDir, "Login shows an error"))
	assertTestFile(t, filepath.Join(reportDir, "screen-2.png"), "b")
}

func Test_exportTestAttachments_noAttachmentsKeepsTheReport(t *testing.T) {
	xml := junitXML("", "no marker here")
	projectDir, reportDir := setUpReport(t, xml)

	exportTestAttachments(log.NewLogger(), fileutil.NewFileManager(), projectDir, reportDir)

	assertTestFile(t, filepath.Join(reportDir, testResultFileName), xml)
}

func Test_exportTestAttachments_missingFile(t *testing.T) {
	xml := junitXML("", "[[ATTACHMENT|build/missing.png]]")
	projectDir, reportDir := setUpReport(t, xml)
	var logs bytes.Buffer

	exportTestAttachments(log.NewLogger(log.WithOutput(&logs)), fileutil.NewFileManager(), projectDir, reportDir)

	assert.Contains(t, logs.String(), filepath.Join(projectDir, "build", "missing.png"))
	assertTestFile(t, filepath.Join(reportDir, testResultFileName), xml)
}

func setUpReport(t *testing.T, xml string) (projectDir, reportDir string) {
	t.Helper()
	return t.TempDir(), writeReport(t, xml)
}

func writeReport(t *testing.T, xml string) string {
	t.Helper()
	reportDir := t.TempDir()
	writeTestFile(t, filepath.Join(reportDir, testResultFileName), xml)
	return reportDir
}

func attachmentsOf(t *testing.T, reportDir, testName string) []string {
	t.Helper()
	report, err := readJUnitReport(filepath.Join(reportDir, testResultFileName))
	require.NoError(t, err)

	var values []string
	found := false
	for _, testCase := range report.TestSuites[0].TestCases {
		if testCase.Name != testName {
			continue
		}
		found = true
		if testCase.Properties == nil {
			continue
		}
		for _, property := range testCase.Properties.Property {
			if strings.HasPrefix(property.Name, attachmentPropertyPrefix) {
				values = append(values, property.Value)
			}
		}
	}
	require.True(t, found, "no test case %q", testName)
	return values
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
