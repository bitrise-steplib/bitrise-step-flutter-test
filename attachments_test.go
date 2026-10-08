package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/testattachment"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const loginClassName = ".Users.vagrant.git.test.login"

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

func goldenFailureOutput(golden, failuresDir string) string {
	// The wrapped shape flutter_test prints a golden failure in.
	return `══╡ EXCEPTION CAUGHT BY FLUTTER TEST FRAMEWORK ╞═══
The following assertion was thrown while running async test code:
Golden "` + golden + `": Pixel test failed, 1.00%, 4800px diff detected.
Failure feedback can be found at
` + failuresDir + `

When the exception was thrown, this was the stack:`
}

func Test_exportGoldenFailureImages(t *testing.T) {
	failuresDir := filepath.Join(t.TempDir(), "test", "failures")
	reportDir := writeReport(t, junitXML("Test failed. See exception logs above.", goldenFailureOutput("goldens/login.png", failuresDir)))
	for _, suffix := range []string{"masterImage", "testImage", "isolatedDiff", "maskedDiff"} {
		writeTestFile(t, filepath.Join(failuresDir, "login_"+suffix+".png"), suffix)
	}

	exportGoldenFailureImages(log.NewLogger(), fileutil.NewFileManager(), reportDir)

	assert.Equal(t, []string{"login_masterImage.png", "login_testImage.png", "login_isolatedDiff.png", "login_maskedDiff.png"}, attachmentsOf(t, reportDir, "Login shows an error"))
	assert.Empty(t, attachmentsOf(t, reportDir, "Login accepts a valid password"))
	assertTestFile(t, filepath.Join(reportDir, "login_maskedDiff.png"), "maskedDiff")
}

func Test_exportGoldenFailureImages_sizeMismatch(t *testing.T) {
	failuresDir := filepath.Join(t.TempDir(), "test", "failures")
	output := `Golden "goldens/login.png": Pixel test failed, image sizes do not match.
Master Image: 800 X 600
Test Image: 400 X 300
Failure feedback can be found at ` + failuresDir
	reportDir := writeReport(t, junitXML(output, ""))
	// A size mismatch writes only two images.
	writeTestFile(t, filepath.Join(failuresDir, "login_masterImage.png"), "master")
	writeTestFile(t, filepath.Join(failuresDir, "login_testImage.png"), "test")
	var logs bytes.Buffer

	exportGoldenFailureImages(log.NewLogger(log.WithOutput(&logs)), fileutil.NewFileManager(), reportDir)

	assert.Equal(t, []string{"login_masterImage.png", "login_testImage.png"}, attachmentsOf(t, reportDir, "Login shows an error"))
	assert.NotContains(t, logs.String(), "Skipping")
}

func Test_exportGoldenFailureImages_goldenNameWithSpace(t *testing.T) {
	failuresDir := filepath.Join(t.TempDir(), "test", "failures")
	// flutter_test prints the golden as a URI but names the images after the decoded file name.
	reportDir := writeReport(t, junitXML("", goldenFailureOutput("goldens/login%20screen.png", failuresDir)))
	writeTestFile(t, filepath.Join(failuresDir, "login screen_masterImage.png"), "master")

	exportGoldenFailureImages(log.NewLogger(), fileutil.NewFileManager(), reportDir)

	assert.Equal(t, []string{"login screen_masterImage.png"}, attachmentsOf(t, reportDir, "Login shows an error"))
}

func Test_exportGoldenFailureImages_sameNamesFromTwoFolders(t *testing.T) {
	dir := t.TempDir()
	firstDir := filepath.Join(dir, "a", "failures")
	secondDir := filepath.Join(dir, "b", "failures")
	reportDir := writeReport(t, junitXML(goldenFailureOutput("goldens/Screen.png", secondDir), goldenFailureOutput("goldens/screen.png", firstDir)))
	writeTestFile(t, filepath.Join(firstDir, "screen_masterImage.png"), "a")
	writeTestFile(t, filepath.Join(secondDir, "Screen_masterImage.png"), "b")

	exportGoldenFailureImages(log.NewLogger(), fileutil.NewFileManager(), reportDir)

	// The names differ only in case, which the macOS file system treats as the same file.
	assert.Equal(t, []string{"screen_masterImage.png", "Screen_masterImage-2.png"}, attachmentsOf(t, reportDir, "Login shows an error"))
	assertTestFile(t, filepath.Join(reportDir, "Screen_masterImage-2.png"), "b")
}

func Test_exportGoldenFailureImages_noImagesKeepsTheReport(t *testing.T) {
	xml := junitXML("", goldenFailureOutput("goldens/login.png", filepath.Join(t.TempDir(), "failures")))
	reportDir := writeReport(t, xml)
	var logs bytes.Buffer

	exportGoldenFailureImages(log.NewLogger(log.WithOutput(&logs)), fileutil.NewFileManager(), reportDir)

	assert.Empty(t, logs.String())
	assertTestFile(t, filepath.Join(reportDir, testResultFileName), xml)
}

func Test_exportConventionAttachments(t *testing.T) {
	projectDir := t.TempDir()
	reportDir := writeReport(t, junitXML("", ""))
	attachmentName := loginClassName + "__Login shows an error__1.png"
	writeTestFile(t, filepath.Join(projectDir, "build", "screenshots", attachmentName), "screenshot")

	exportConventionAttachments(log.NewLogger(), newTestCollector(), projectDir, t.TempDir(), filepath.Join(reportDir, testResultFileName), reportDir)

	assertTestFile(t, filepath.Join(reportDir, attachmentName), "screenshot")
}

func Test_exportConventionAttachments_invalidXML(t *testing.T) {
	projectDir := t.TempDir()
	reportDir := writeReport(t, "not xml")
	attachmentName := loginClassName + "__Login shows an error__1.png"
	writeTestFile(t, filepath.Join(projectDir, attachmentName), "screenshot")
	var logs bytes.Buffer

	exportConventionAttachments(log.NewLogger(log.WithOutput(&logs)), newTestCollector(), projectDir, t.TempDir(), filepath.Join(reportDir, testResultFileName), reportDir)

	assert.Contains(t, logs.String(), "Failed to read test cases")
	assert.NoFileExists(t, filepath.Join(reportDir, attachmentName))
}

func Test_exportConventionAttachments_missingDeployDir(t *testing.T) {
	projectDir := t.TempDir()
	reportDir := writeReport(t, junitXML("", ""))
	attachmentName := loginClassName + "__Login shows an error__1.png"
	writeTestFile(t, filepath.Join(projectDir, attachmentName), "screenshot")
	var logs bytes.Buffer

	exportConventionAttachments(log.NewLogger(log.WithOutput(&logs)), newTestCollector(), projectDir, "", filepath.Join(reportDir, testResultFileName), reportDir)

	assert.Contains(t, logs.String(), "BITRISE_TEST_DEPLOY_DIR is not set")
	assertTestFile(t, filepath.Join(reportDir, attachmentName), "screenshot")
}

func newTestCollector() testattachment.Collector {
	return testattachment.NewCollector(command.NewFactory(env.NewRepository()), fileutil.NewFileManager())
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
