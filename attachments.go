package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/bitrise-io/go-android/v2/testresult/junitxml"
	"github.com/bitrise-io/go-steputils/v2/testasset"
	"github.com/bitrise-io/go-steputils/v2/testreport"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
)

const attachmentPropertyPrefix = "attachment_"

var (
	attachmentMarkerPattern = regexp.MustCompile(`\[\[ATTACHMENT\|([^\]]+)\]\]`)
	goldenFailurePattern    = regexp.MustCompile(`Golden "([^"]+)": `)
	// The test framework wraps the message, so the folder is usually on the next line.
	failureFeedbackPattern   = regexp.MustCompile(`Failure feedback can be found at\s+(\S.*)`)
	goldenFailureImageSuffix = []string{"masterImage", "testImage", "isolatedDiff", "maskedDiff"}
)

// testRef identifies a test case of the JUnit XML the way tojunit writes it. run tells apart the
// occurrences of the same class name and name, in the order the tests started.
type testRef struct {
	className string
	name      string
	run       int
}

type testAttachments struct {
	test  testRef
	files []string
}

// exportTestAttachments links the files a test reported in the `flutter test` JSON events to its
// test case: files printed as [[ATTACHMENT|<path>]] and the images of a failed golden test. The
// files are copied next to the exported JUnit XML and referenced by attachment_N properties.
// Problems are logged as warnings: the test result itself is already exported.
func exportTestAttachments(logger log.Logger, fileManager fileutil.FileManager, events []byte, projectDir, reportDir string) {
	attachments, err := parseTestAttachments(events, projectDir)
	if err != nil {
		logger.Warnf("Failed to read the test events, attachments are not exported: %s", err)
		return
	}
	if len(attachments) == 0 {
		return
	}

	junitPath := filepath.Join(reportDir, testResultFileName)
	report, err := readJUnitReport(junitPath)
	if err != nil {
		logger.Warnf("Failed to read test cases, attachments are not exported: %s", err)
		return
	}
	usedNames, err := fileNamesIn(reportDir)
	if err != nil {
		logger.Warnf("Failed to read the test report folder, attachments are not exported: %s", err)
		return
	}

	testCases := indexTestCases(&report)
	exported := 0
	for _, a := range attachments {
		testCase, ok := testCases[a.test]
		if !ok {
			logger.Warnf("Skipping attachments of %q: no such test case in the test report", a.test.name)
			continue
		}
		for _, file := range a.files {
			fileName, err := copyAttachment(fileManager, file, reportDir, usedNames)
			if err != nil {
				logger.Warnf("Skipping attachment %s of %q: %s", file, a.test.name, err)
				continue
			}
			addAttachmentProperty(testCase, fileName)
			exported++
		}
	}
	if exported == 0 {
		return
	}

	if err := writeJUnitReport(junitPath, report); err != nil {
		logger.Warnf("Failed to link attachments in the test report: %s", err)
		return
	}
	logger.Donef("Exported %d test attachments.", exported)
}

// parseTestAttachments reads the attachments of each test from the JSON events. Relative marker
// paths are resolved from projectDir, the working directory of `flutter test`.
func parseTestAttachments(events []byte, projectDir string) ([]testAttachments, error) {
	suitePaths := map[int]string{}
	tests := map[int]testRef{}
	runs := map[testRef]int{}
	hidden := map[int]bool{}
	filesByTest := map[int][]string{}
	var order []int

	reader := bufio.NewReader(bytes.NewReader(events))
	for {
		line, err := reader.ReadBytes('\n')
		var event struct {
			Type  string `json:"type"`
			Suite struct {
				ID   int    `json:"id"`
				Path string `json:"path"`
			} `json:"suite"`
			Test struct {
				ID      int    `json:"id"`
				Name    string `json:"name"`
				SuiteID int    `json:"suiteID"`
			} `json:"test"`
			TestID  int    `json:"testID"`
			Hidden  bool   `json:"hidden"`
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		// The legacy --machine output can contain lines that are not events.
		if json.Unmarshal(line, &event) == nil {
			switch event.Type {
			case "suite":
				suitePaths[event.Suite.ID] = event.Suite.Path
			case "testStart":
				test := testRef{className: className(suitePaths[event.Test.SuiteID]), name: event.Test.Name}
				run := runs[test]
				runs[test]++
				test.run = run
				tests[event.Test.ID] = test
			case "testDone":
				// Loading, setUpAll and tearDownAll run as hidden tests that tojunit leaves out.
				hidden[event.TestID] = event.Hidden
			case "print", "error":
				files := filesInMessage(event.Message+event.Error, projectDir)
				if len(files) == 0 {
					break
				}
				if _, seen := filesByTest[event.TestID]; !seen {
					order = append(order, event.TestID)
				}
				for _, file := range files {
					if !slices.Contains(filesByTest[event.TestID], file) {
						filesByTest[event.TestID] = append(filesByTest[event.TestID], file)
					}
				}
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	var attachments []testAttachments
	for _, testID := range order {
		test, ok := tests[testID]
		if !ok || hidden[testID] {
			continue
		}
		attachments = append(attachments, testAttachments{test: test, files: filesByTest[testID]})
	}
	return attachments, nil
}

// filesInMessage returns the files a print or error message reports: the paths of
// [[ATTACHMENT|<path>]] markers, and the images a failed golden test wrote. A golden failure
// names its failures folder, and the images in it are named after the golden file.
func filesInMessage(message, projectDir string) []string {
	var files []string
	for _, match := range attachmentMarkerPattern.FindAllStringSubmatch(message, -1) {
		file := strings.TrimSpace(match[1])
		if !filepath.IsAbs(file) {
			file = filepath.Join(projectDir, file)
		}
		files = append(files, file)
	}

	feedback := failureFeedbackPattern.FindStringSubmatch(message)
	if feedback == nil {
		return files
	}
	failuresDir := strings.TrimSpace(feedback[1])
	if unescaped, err := url.PathUnescape(failuresDir); err == nil {
		failuresDir = unescaped
	}
	for _, match := range goldenFailurePattern.FindAllStringSubmatch(message, -1) {
		golden := path.Base(match[1])
		stem := strings.TrimSuffix(golden, path.Ext(golden))
		// A size mismatch writes only some of the images.
		for _, suffix := range goldenFailureImageSuffix {
			if image := filepath.Join(failuresDir, stem+"_"+suffix+".png"); exists(image) {
				files = append(files, image)
			}
		}
	}
	return files
}

// className is the class name tojunit writes for a test file: its path without "_test.dart" (or
// ".dart"), with path separators replaced by "." and "-" by "_".
func className(testFile string) string {
	name := strings.TrimSuffix(testFile, "_test.dart")
	if name == testFile {
		name = strings.TrimSuffix(testFile, ".dart")
	}
	name = strings.NewReplacer("/", ".", `\`, ".").Replace(name)
	return strings.ReplaceAll(name, "-", "_")
}

func indexTestCases(report *testreport.TestReport) map[testRef]*testreport.TestCase {
	testCases := map[testRef]*testreport.TestCase{}
	runs := map[testRef]int{}
	var addSuite func(suite *testreport.TestSuite)
	addSuite = func(suite *testreport.TestSuite) {
		for i := range suite.TestCases {
			testCase := &suite.TestCases[i]
			test := testRef{className: testCase.ClassName, name: testCase.Name}
			run := runs[test]
			runs[test]++
			test.run = run
			testCases[test] = testCase
		}
		for i := range suite.TestSuites {
			addSuite(&suite.TestSuites[i])
		}
	}
	for i := range report.TestSuites {
		addSuite(&report.TestSuites[i])
	}
	return testCases
}

// copyAttachment copies the file to the top of the report folder under a name no other file of the
// report uses, and returns that name.
func copyAttachment(fileManager fileutil.FileManager, src, reportDir string, usedNames map[string]bool) (string, error) {
	info, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a folder", src)
	}
	if !testasset.IsSupportedAssetType(src) {
		return "", fmt.Errorf("unsupported file type, use one of %s", strings.Join(testasset.AssetTypes, ", "))
	}

	fileName := uniqueFileName(filepath.Base(src), usedNames)
	if err := fileManager.CopyFile(src, filepath.Join(reportDir, fileName), &fileutil.CopyOptions{}); err != nil {
		return "", err
	}
	usedNames[fileName] = true
	return fileName, nil
}

func uniqueFileName(fileName string, usedNames map[string]bool) string {
	ext := filepath.Ext(fileName)
	stem := strings.TrimSuffix(fileName, ext)
	candidate := fileName
	for i := 2; usedNames[candidate]; i++ {
		candidate = stem + "-" + strconv.Itoa(i) + ext
	}
	return candidate
}

func fileNamesIn(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, entry := range entries {
		names[entry.Name()] = true
	}
	return names, nil
}

func addAttachmentProperty(testCase *testreport.TestCase, fileName string) {
	if testCase.Properties == nil {
		testCase.Properties = &testreport.Properties{}
	}
	next := 0
	for _, property := range testCase.Properties.Property {
		suffix, ok := strings.CutPrefix(property.Name, attachmentPropertyPrefix)
		if !ok {
			continue
		}
		if n, err := strconv.Atoi(suffix); err == nil && n >= next {
			next = n + 1
		}
	}
	testCase.Properties.Property = append(testCase.Properties.Property, testreport.Property{
		Name:  attachmentPropertyPrefix + strconv.Itoa(next),
		Value: fileName,
	})
}

func readJUnitReport(path string) (testreport.TestReport, error) {
	var converter junitxml.Converter
	if !converter.Detect([]string{path}) {
		return testreport.TestReport{}, fmt.Errorf("%s is not a JUnit XML", path)
	}
	return converter.Convert()
}

func writeJUnitReport(path string, report testreport.TestReport) error {
	data, err := xml.MarshalIndent(report, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append([]byte(xml.Header), data...), 0o644)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
