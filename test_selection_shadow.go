package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/bitrise-steplib/bitrise-step-test-selection/selector"
)

// startTestSelectionShadow works out which packages a change-based test selection
// would have run. It never changes what the step runs, and it only acts when the
// backend injects a shadow configuration; otherwise it returns nil.
func startTestSelectionShadow(cfg config) *selector.Shadow {
	sh, ok := selector.StartShadow(selector.ShadowOptions{
		Step:    "flutter-test",
		Config:  selector.Config{ProjectType: "flutter"},
		WorkDir: cfg.ProjectLocation,
		Keep: func(t selector.Test, repoDir string) bool {
			rel, ok := selector.RelPath(repoDir, cfg.ProjectLocation)
			return ok && t.Ident.PackageDir == rel
		},
		Logf: func(format string, v ...interface{}) {
			fmt.Println()
			logger.Infof(format, v...)
		},
		Debugf: logger.Debugf,
	})
	if !ok {
		return nil
	}
	return sh
}

// finishTestSelectionShadow reports the package's test results so the backend can tell
// what a selection would have saved and whether it would have skipped a failure.
func finishTestSelectionShadow(sh *selector.Shadow, cfg config, testRun time.Duration) {
	if sh == nil {
		return
	}
	sh.Finish(packageOutcomes(sh.RepoDir(), cfg, testRun), testRun)
}

// packageOutcomes reads the JUnit report the step converted from flutter test's JSON
// output. The step tests one package, so its whole test run is that package's duration,
// compilation included. nil means the results could not be read.
func packageOutcomes(repoDir string, cfg config, testRun time.Duration) []selector.UnitOutcome {
	key, ok := selector.RelPath(repoDir, cfg.ProjectLocation)
	if !ok {
		return nil
	}
	outcome, err := selector.JUnitOutcome(key, []string{filepath.Join(cfg.ProjectLocation, testResultFileName)})
	if err != nil {
		logger.Debugf("Test selection shadow run: cannot read the test results: %s", err)
		return nil
	}
	outcome.Duration = testRun
	return []selector.UnitOutcome{outcome}
}
