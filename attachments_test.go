package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/testreport"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const loginClassName = ".Users.vagrant.git.test.login_flow"

// The shape tojunit writes for test/login-flow_test.dart in /Users/vagrant/git.
const loginJUnitXML = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite errors="0" failures="1" tests="3" skipped="0" name=".Users.vagrant.git.test.login_flow" timestamp="2026-10-08T09:00:00">
    <testcase classname=".Users.vagrant.git.test.login_flow" name="Login shows an error" time="0.1"/>
    <testcase classname=".Users.vagrant.git.test.login_flow" name="Login accepts a valid password" time="0.1"/>
    <testcase classname=".Users.vagrant.git.test.login_flow" name="Login accepts a valid password" time="0.1"/>
  </testsuite>
</testsuites>
`

const loginEventsHeader = `{"suite":{"id":0,"platform":"vm","path":"/Users/vagrant/git/test/login-flow_test.dart"},"type":"suite","time":0}
{"test":{"id":1,"name":"Login shows an error","suiteID":0},"type":"testStart","time":1}
{"test":{"id":2,"name":"Login accepts a valid password","suiteID":0},"type":"testStart","time":2}
{"test":{"id":3,"name":"Login accepts a valid password","suiteID":0},"type":"testStart","time":3}
`

func Test_exportTestAttachments_marker(t *testing.T) {
	projectDir, reportDir := setUpReport(t)
	writeTestFile(t, filepath.Join(projectDir, "build", "shots", "error.png"), "screenshot")
	events := loginEventsHeader + `{"testID":1,"messageType":"print","message":"[[ATTACHMENT|build/shots/error.png]]","type":"print","time":4}
`

	exportTestAttachments(log.NewLogger(), fileutil.NewFileManager(), []byte(events), projectDir, reportDir)

	assertTestFile(t, filepath.Join(reportDir, "error.png"), "screenshot")
	assert.Equal(t, []string{"error.png"}, attachmentsOf(t, reportDir, "Login shows an error", 0))
	assert.Empty(t, attachmentsOf(t, reportDir, "Login accepts a valid password", 0))
}

func Test_exportTestAttachments_secondRunOfTheSameTest(t *testing.T) {
	projectDir, reportDir := setUpReport(t)
	writeTestFile(t, filepath.Join(projectDir, "valid.png"), "screenshot")
	events := loginEventsHeader + `{"testID":3,"messageType":"print","message":"[[ATTACHMENT|valid.png]]","type":"print","time":4}
`

	exportTestAttachments(log.NewLogger(), fileutil.NewFileManager(), []byte(events), projectDir, reportDir)

	assert.Empty(t, attachmentsOf(t, reportDir, "Login accepts a valid password", 0))
	assert.Equal(t, []string{"valid.png"}, attachmentsOf(t, reportDir, "Login accepts a valid password", 1))
}

func Test_exportTestAttachments_goldenFailure(t *testing.T) {
	projectDir, reportDir := setUpReport(t)
	failuresDir := filepath.Join(projectDir, "test", "failures")
	// A size mismatch writes only these two images.
	writeTestFile(t, filepath.Join(failuresDir, "login_masterImage.png"), "master")
	writeTestFile(t, filepath.Join(failuresDir, "login_testImage.png"), "test")
	// The wrapped shape flutter_test prints a golden failure in.
	message := `══╡ EXCEPTION CAUGHT BY FLUTTER TEST FRAMEWORK ╞═══\nThe following assertion was thrown while running async test code:\nGolden \"goldens/login.png\": Pixel test failed, image sizes do not match.\nMaster Image: 800 X 600\nTest Image: 400 X 300\nFailure feedback can be found at\n` + failuresDir + `\n\nWhen the exception was thrown, this was the stack:`
	events := loginEventsHeader + `{"testID":1,"messageType":"print","message":"` + message + `","type":"print","time":4}
{"testID":1,"error":"Test failed. See exception logs above.","stackTrace":"","isFailure":false,"type":"error","time":5}
`
	var logs bytes.Buffer

	exportTestAttachments(log.NewLogger(log.WithOutput(&logs)), fileutil.NewFileManager(), []byte(events), projectDir, reportDir)

	assert.Equal(t, []string{"login_masterImage.png", "login_testImage.png"}, attachmentsOf(t, reportDir, "Login shows an error", 0))
	assert.NotContains(t, logs.String(), "Skipping")
}

func Test_exportTestAttachments_sameFileNameInTwoTests(t *testing.T) {
	projectDir, reportDir := setUpReport(t)
	writeTestFile(t, filepath.Join(projectDir, "a", "screen.png"), "a")
	writeTestFile(t, filepath.Join(projectDir, "b", "screen.png"), "b")
	events := loginEventsHeader + `{"testID":1,"message":"[[ATTACHMENT|a/screen.png]]","type":"print"}
{"testID":2,"message":"[[ATTACHMENT|b/screen.png]]","type":"print"}
`

	exportTestAttachments(log.NewLogger(), fileutil.NewFileManager(), []byte(events), projectDir, reportDir)

	assert.Equal(t, []string{"screen.png"}, attachmentsOf(t, reportDir, "Login shows an error", 0))
	assert.Equal(t, []string{"screen-2.png"}, attachmentsOf(t, reportDir, "Login accepts a valid password", 0))
	assertTestFile(t, filepath.Join(reportDir, "screen-2.png"), "b")
}

func Test_exportTestAttachments_noAttachmentsKeepsTheReport(t *testing.T) {
	projectDir, reportDir := setUpReport(t)
	events := loginEventsHeader + `{"testID":1,"message":"no marker here","type":"print"}
`

	exportTestAttachments(log.NewLogger(), fileutil.NewFileManager(), []byte(events), projectDir, reportDir)

	assertTestFile(t, filepath.Join(reportDir, testResultFileName), loginJUnitXML)
}

func Test_exportTestAttachments_missingFile(t *testing.T) {
	projectDir, reportDir := setUpReport(t)
	events := loginEventsHeader + `{"testID":1,"message":"[[ATTACHMENT|build/missing.png]]","type":"print"}
`
	var logs bytes.Buffer

	exportTestAttachments(log.NewLogger(log.WithOutput(&logs)), fileutil.NewFileManager(), []byte(events), projectDir, reportDir)

	assert.Contains(t, logs.String(), filepath.Join(projectDir, "build", "missing.png"))
	assertTestFile(t, filepath.Join(reportDir, testResultFileName), loginJUnitXML)
}

func Test_exportTestAttachments_hiddenTest(t *testing.T) {
	projectDir, reportDir := setUpReport(t)
	writeTestFile(t, filepath.Join(projectDir, "setup.png"), "screenshot")
	events := loginEventsHeader + `{"test":{"id":4,"name":"(setUpAll)","suiteID":0},"type":"testStart"}
{"testID":4,"message":"[[ATTACHMENT|setup.png]]","type":"print"}
{"testID":4,"result":"success","skipped":false,"hidden":true,"type":"testDone"}
`
	var logs bytes.Buffer

	exportTestAttachments(log.NewLogger(log.WithOutput(&logs)), fileutil.NewFileManager(), []byte(events), projectDir, reportDir)

	assert.Empty(t, logs.String())
	assertTestFile(t, filepath.Join(reportDir, testResultFileName), loginJUnitXML)
}

func Test_exportTestAttachments_legacyOutputLines(t *testing.T) {
	projectDir, reportDir := setUpReport(t)
	writeTestFile(t, filepath.Join(projectDir, "error.png"), "screenshot")
	events := "Running \"flutter pub get\" in app...\n" + loginEventsHeader + `{"testID":1,"message":"[[ATTACHMENT|error.png]]","type":"print"}`

	exportTestAttachments(log.NewLogger(), fileutil.NewFileManager(), []byte(events), projectDir, reportDir)

	assert.Equal(t, []string{"error.png"}, attachmentsOf(t, reportDir, "Login shows an error", 0))
}

func Test_className(t *testing.T) {
	assert.Equal(t, loginClassName, className("/Users/vagrant/git/test/login-flow_test.dart"))
	assert.Equal(t, ".bitrise.src.test.helpers.widget_utils", className("/bitrise/src/test/helpers/widget-utils.dart"))
}

func setUpReport(t *testing.T) (projectDir, reportDir string) {
	t.Helper()
	projectDir = t.TempDir()
	reportDir = t.TempDir()
	writeTestFile(t, filepath.Join(reportDir, testResultFileName), loginJUnitXML)
	return projectDir, reportDir
}

func attachmentsOf(t *testing.T, reportDir, testName string, run int) []string {
	t.Helper()
	report, err := readJUnitReport(filepath.Join(reportDir, testResultFileName))
	require.NoError(t, err)
	testCase := indexTestCases(&report)[testRef{className: loginClassName, name: testName, run: run}]
	require.NotNil(t, testCase)
	return attachmentValues(testCase)
}

func attachmentValues(testCase *testreport.TestCase) []string {
	var values []string
	if testCase.Properties == nil {
		return nil
	}
	for _, property := range testCase.Properties.Property {
		if strings.HasPrefix(property.Name, attachmentPropertyPrefix) {
			values = append(values, property.Value)
		}
	}
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
