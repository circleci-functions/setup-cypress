//go:build acceptance

package acceptance_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/circleci-functions/toolkit/sdk"
	"github.com/circleci-functions/toolkit/sdk/sdktest"
)

// site serves a home page whose heading is heading, which the test project's
// spec expects to be "home".
func site(t *testing.T, heading string) string {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "<html><body><h1>%s</h1></body></html>", heading)
	}))
	t.Cleanup(s.Close)
	return s.URL
}

// needTestsuite skips a test that runs the suite where the circleci CLI has
// no testsuite, which a CircleCI job always has.
func needTestsuite(t *testing.T) {
	t.Helper()
	if err := exec.Command("circleci", "testsuite", "--help").Run(); err != nil {
		if sdk.InCircleCIJob() {
			t.Fatalf("circleci testsuite is not available in this job: %v", err)
		}
		t.Skipf("circleci testsuite is not available: %v (install it with `circleci extension install testsuite`)", err)
	}
}

// suite names a test's suite uniquely within a workflow. testsuite takes a
// second run of a suite in a workflow as a rerun, and skips the specs that
// passed the first time, so the tests and the jobs running them each need
// their own.
func suite(name string) string {
	if job := os.Getenv("CIRCLE_JOB"); job != "" {
		return name + "-" + job
	}
	return name
}

// runEnv is an env sharing one Cypress cache folder across tests, so the
// binary is downloaded once rather than per test, and the in-memory cache, so
// the tests can see the artifacts stored. The project is made a git
// repository, as a checkout is, since testsuite finds the project's root by
// its .git. The circleci CLI finds its extensions under the XDG data
// directory, which defaults to one in the home directory the sandbox
// replaces, so the real one is passed on.
func runEnv(t *testing.T) *env {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("testsuite's default Cypress suite does not run on Windows: it passes spec paths to bash with backslashes, which bash drops")
	}
	needTestsuite(t)
	e := newEnv(t)
	e.sandbox = sdktest.NewSandbox(t)
	git := exec.Command("git", "init", "--quiet")
	git.Dir = e.project
	out, err := git.CombinedOutput()
	assert.NilError(t, err, string(out))
	e.folder = sharedFolder
	e.vars["CYPRESS_CACHE_FOLDER"] = sharedFolder
	e.vars["XDG_DATA_HOME"] = dataHome(t)
	return e
}

func dataHome(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	assert.NilError(t, err)
	return filepath.Join(home, ".local", "share")
}

func TestRun_PassesAgainstTheSite(t *testing.T) {
	e := runEnv(t)

	out, err := e.run(t, "run", "--suite="+suite("passes"), "--working-directory="+e.project, "--base-url="+site(t, "home"))
	assert.NilError(t, err, out)
	assert.Check(t, cmp.Contains(out, "home.cy.js"))
}

func TestRun_FailsWhenASpecFails(t *testing.T) {
	e := runEnv(t)

	out, err := e.run(t, "run", "--suite="+suite("fails"), "--working-directory="+e.project, "--base-url="+site(t, "broken"))
	assert.Check(t, err != nil, out)
	assert.Check(t, cmp.Contains(out, "expected '<h1>' to have text 'home', but the text was 'broken'"))
	assert.Check(t, cmp.Contains(out, `circleci testsuite run "`+suite("fails")+`" failed`))

	stored := map[string]string{}
	for _, a := range e.sandbox.Cache.StoredArtifacts() {
		stored[a.Destination] = a.Path
	}
	shots, _ := filepath.Glob(filepath.Join(stored["cypress/screenshots"], "home.cy.js", "*.png"))
	assert.Check(t, cmp.Len(shots, 1), "the failure's screenshot")
	reports, _ := filepath.Glob(filepath.Join(stored["cypress/results"], "*", "result-*.xml"))
	assert.Check(t, cmp.Len(reports, 1), "the spec's JUnit report")
}

func TestRun_DefaultsToTheDeployment(t *testing.T) {
	e := runEnv(t)
	e.vars["DEPLOYMENT_URL"] = site(t, "home")

	out, err := e.run(t, "--suite="+suite("deployment"), "--working-directory="+e.project)
	assert.NilError(t, err, out)
	assert.Check(t, cmp.Contains(out, "testing "+e.vars["DEPLOYMENT_URL"]))
}
