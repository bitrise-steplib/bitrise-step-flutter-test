package main

import (
	"encoding/xml"
	"fmt"
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

// exportTestAttachments links the files a test reported in its output to its test case in the
// exported JUnit XML: files printed as [[ATTACHMENT|<path>]] and the images of a failed golden
// test. tojunit writes a test's prints to its system-out, and its errors to its failure or error.
// The files are copied next to the XML and referenced by attachment_N properties.
// Problems are logged as warnings: the test result itself is already exported.
func exportTestAttachments(logger log.Logger, fileManager fileutil.FileManager, projectDir, reportDir string) {
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

	exported := 0
	forEachTestCase(&report, func(testCase *testreport.TestCase) {
		for _, file := range filesInOutput(testCase, projectDir) {
			fileName, err := copyAttachment(fileManager, file, reportDir, usedNames)
			if err != nil {
				logger.Warnf("Skipping attachment %s of %q: %s", file, testCase.Name, err)
				continue
			}
			addAttachmentProperty(testCase, fileName)
			exported++
		}
	})
	if exported == 0 {
		return
	}

	if err := writeJUnitReport(junitPath, report); err != nil {
		logger.Warnf("Failed to link attachments in the test report: %s", err)
		return
	}
	logger.Donef("Exported %d test attachments.", exported)
}

func forEachTestCase(report *testreport.TestReport, fn func(*testreport.TestCase)) {
	var walk func(suite *testreport.TestSuite)
	walk = func(suite *testreport.TestSuite) {
		for i := range suite.TestCases {
			fn(&suite.TestCases[i])
		}
		for i := range suite.TestSuites {
			walk(&suite.TestSuites[i])
		}
	}
	for i := range report.TestSuites {
		walk(&report.TestSuites[i])
	}
}

// filesInOutput returns the files a test case reported: the paths of [[ATTACHMENT|<path>]]
// markers, resolved from projectDir (the working directory of `flutter test`), and the images a
// failed golden test wrote. A golden failure names its failures folder, and the images in it are
// named after the golden file.
func filesInOutput(testCase *testreport.TestCase, projectDir string) []string {
	var output []string
	if testCase.SystemOut != nil {
		output = append(output, testCase.SystemOut.Value)
	}
	if testCase.Failure != nil {
		output = append(output, testCase.Failure.Value)
	}
	if testCase.Error != nil {
		output = append(output, testCase.Error.Value)
	}

	var files []string
	add := func(file string) {
		if !slices.Contains(files, file) {
			files = append(files, file)
		}
	}
	for _, text := range output {
		for _, match := range attachmentMarkerPattern.FindAllStringSubmatch(text, -1) {
			file := strings.TrimSpace(match[1])
			if !filepath.IsAbs(file) {
				file = filepath.Join(projectDir, file)
			}
			add(file)
		}

		feedback := failureFeedbackPattern.FindStringSubmatch(text)
		if feedback == nil {
			continue
		}
		failuresDir := strings.TrimSpace(feedback[1])
		if unescaped, err := url.PathUnescape(failuresDir); err == nil {
			failuresDir = unescaped
		}
		for _, match := range goldenFailurePattern.FindAllStringSubmatch(text, -1) {
			golden := path.Base(match[1])
			stem := strings.TrimSuffix(golden, path.Ext(golden))
			// A size mismatch writes only some of the images, or none on older Flutter versions.
			for _, suffix := range goldenFailureImageSuffix {
				if image := filepath.Join(failuresDir, stem+"_"+suffix+".png"); exists(image) {
					add(image)
				}
			}
		}
	}
	return files
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
