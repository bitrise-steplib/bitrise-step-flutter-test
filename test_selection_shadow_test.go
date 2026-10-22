package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/bitrise-steplib/bitrise-step-test-selection/selector"
)

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// coreChanged commits app ← core and changes core on a branch.
func coreChanged(t *testing.T) string {
	root := t.TempDir()
	writeFile(t, root, "app/pubspec.yaml", "name: app\ndependencies:\n  core:\n    path: ../core\n")
	writeFile(t, root, "app/test/app_test.dart", "")
	writeFile(t, root, "core/pubspec.yaml", "name: core\n")
	writeFile(t, root, "core/lib/core.dart", "")
	writeFile(t, root, "core/test/core_test.dart", "")
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", "init")
	runGit(t, root, "checkout", "-q", "-b", "change")
	writeFile(t, root, "core/lib/core.dart", "// changed\n")
	runGit(t, root, "commit", "-q", "-am", "change core")
	return root
}

func TestStartTestSelectionShadowNeedsTheConfiguration(t *testing.T) {
	t.Setenv(selector.ConfigEnvKey, "")
	if sh := startTestSelectionShadow(config{ProjectLocation: filepath.Join(coreChanged(t), "app")}); sh != nil {
		t.Fatalf("started without a shadow configuration: %+v", sh.Report)
	}
	finishTestSelectionShadow(nil, config{}, time.Second)
}

func TestTestSelectionShadowOutcomes(t *testing.T) {
	root := coreChanged(t)
	t.Setenv(selector.ConfigEnvKey, `{"mode":"shadow","base_ref":"main"}`)
	t.Setenv("ANALYTICS_DISABLED", "true")

	cfg := config{ProjectLocation: filepath.Join(root, "core")}
	sh := startTestSelectionShadow(cfg)
	if sh == nil || sh.Report.Decision != "selected" || sh.Report.Candidates != 1 || len(sh.Report.Selected) != 1 {
		t.Fatalf("start: %+v", sh)
	}

	writeFile(t, cfg.ProjectLocation, testResultFileName, `<testsuites><testsuite name="core">
<testcase name="a" time="0.1"/><testcase name="b" time="0.2"><failure/></testcase>
</testsuite></testsuites>`)
	outcomes := packageOutcomes(sh.RepoDir(), cfg, 12*time.Second)
	if len(outcomes) != 1 || outcomes[0].Key != "core" || outcomes[0].Tests != 2 || outcomes[0].Failed != 1 || outcomes[0].Duration != 12*time.Second {
		t.Fatalf("outcomes %+v", outcomes)
	}

	finishTestSelectionShadow(sh, cfg, 12*time.Second)
	if !sh.Report.OutcomesKnown || sh.Report.MissedFailure || sh.Report.EstimatedSavingMS != 0 {
		t.Errorf("a selected package that failed is no miss and saves nothing: %+v", sh.Report)
	}

	if got := packageOutcomes(sh.RepoDir(), config{ProjectLocation: filepath.Join(root, "app")}, time.Second); got != nil {
		t.Errorf("missing results must give nil outcomes, got %+v", got)
	}
}
