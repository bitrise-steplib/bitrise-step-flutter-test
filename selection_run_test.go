package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-steplib/bitrise-step-flutter-test/testselection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageTestPaths(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "integration_test"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "integration_test", "app_test.dart"), nil, 0o644))
	p := realConfigParser{logger: log.NewLogger()}

	paths, ok := packageTestPaths(p, dir, "integration_test/*_test.dart", true)
	assert.True(t, ok)
	assert.Equal(t, []string{"integration_test/app_test.dart"}, paths)

	_, ok = packageTestPaths(p, dir, "test/*_test.dart", true)
	assert.False(t, ok, "a selected package without matching files must be skipped, not run in full")

	paths, ok = packageTestPaths(p, dir, "test/*_test.dart", false)
	assert.True(t, ok, "without a selection the step keeps its old behaviour")
	assert.Empty(t, paths)

	paths, ok = packageTestPaths(p, dir, "", true)
	assert.True(t, ok)
	assert.Empty(t, paths)
}

func TestSelectedPackageDirsKeepsEachPackageOnce(t *testing.T) {
	t.Setenv("BITRISE_SOURCE_DIR", "/src")
	test := func(id, dir string) testselection.Test {
		return testselection.Test{ID: id, Ecosystem: "flutter", Selected: true, Identifiers: testselection.Identifiers{PackageDir: dir}}
	}
	dirs := selectedPackageDirs([]testselection.Test{
		test("flutter:app:test", "."),
		test("flutter:app:integration_test", "."),
		test("flutter:core:test", "packages/core"),
	})
	assert.Equal(t, []string{"/src", "/src/packages/core"}, dirs)
}

func TestPackageLabelUsesThePathFromTheRoot(t *testing.T) {
	t.Setenv("BITRISE_SOURCE_DIR", "/src")
	assert.Equal(t, "packages/quick_actions/quick_actions_android/example", packageLabel("/src/packages/quick_actions/quick_actions_android/example"))
	assert.Equal(t, "app", packageLabel("/elsewhere/app"))

	testNameSuffix = packageLabel("/src/packages/camera/example")
	defer func() { testNameSuffix = "" }()
	assert.Equal(t, "results_packages_camera_example.json", deployFileName("results.json"))
}
