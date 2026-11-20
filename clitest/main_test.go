// Package clitest_test exercises the setup-cypress CLI contract against a fake
// Cypress CLI in the project's node_modules: the commands it runs, the cache
// entries it saves and restores, and what it exports. Hermetic, so it runs
// under `task test`. The real Cypress is covered by acceptance/.
package clitest_test

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/circleci-functions/toolkit/bashenv"
	"github.com/circleci-functions/toolkit/sdk/sdktest"
	"github.com/circleci-functions/toolkit/testing/compiler"
	"github.com/circleci-functions/toolkit/testing/runner"
)

var binaryPath string

func TestMain(m *testing.M) {
	status, err := runTests(m)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	os.Exit(status)
}

func runTests(m *testing.M) (int, error) {
	p := compiler.NewParallel(1)
	defer p.Cleanup()
	p.Add(compiler.Work{Result: &binaryPath, Name: "setup-cypress", Target: "..", Source: "."})
	if err := p.Run(context.Background()); err != nil {
		return 0, err
	}
	return m.Run(), nil
}

// fakeCypress stands in for the Cypress npm package's CLI. It logs each call,
// with whether the version's binary was already there and the download-skip
// variable it saw, and installs a binary by creating its directory.
const fakeCypress = `#!/bin/sh
folder="${CYPRESS_CACHE_FOLDER:-$HOME/.cache/Cypress}"
binary="$folder/$FAKE_CYPRESS_VERSION"
present=absent
[ -d "$binary" ] && present=present
echo "$* $present skip=$CYPRESS_INSTALL_BINARY" >> "$FAKE_CYPRESS_LOG"
case "$1 $2" in
  "cache path") echo "$folder" ;;
  "install "*) mkdir -p "$binary/Cypress" ;;
  "verify "*) [ -z "$FAKE_CYPRESS_VERIFY_FAILS" ] || { echo "verify failed" >&2; exit 1; } ;;
esac
`

// fakeTestsuite stands in for the circleci CLI's testsuite. It logs the
// arguments, directory and Cypress variables it was run with, saves a
// screenshot and a video into each folder named in FAKE_TESTSUITE_MEDIA, as
// Cypress would, and exits with FAKE_TESTSUITE_EXIT.
const fakeTestsuite = `#!/bin/sh
{
  echo "args=$*"
  echo "dir=$(pwd -P)"
  echo "base=$CYPRESS_BASE_URL"
  echo "folder=$CYPRESS_CACHE_FOLDER"
  echo "skip=$CYPRESS_INSTALL_BINARY"
} >> "$FAKE_TESTSUITE_LOG"
for media in $FAKE_TESTSUITE_MEDIA; do
  mkdir -p "$media" && echo media > "$media/home.cy.js.out"
done
exit "${FAKE_TESTSUITE_EXIT:-0}"
`

const version = "15.7.1"

type env struct {
	sandbox   *sdktest.Sandbox
	project   string
	folder    string
	log       string
	testsuite string
	vars      map[string]string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake Cypress CLI is a shell script")
	}
	e := &env{
		sandbox:   sdktest.NewSandbox(t),
		project:   t.TempDir(),
		folder:    filepath.Join(t.TempDir(), "Cypress"),
		log:       filepath.Join(t.TempDir(), "cypress.log"),
		testsuite: filepath.Join(t.TempDir(), "testsuite.log"),
	}
	pkg := filepath.Join(e.project, "node_modules", "cypress")
	bin := filepath.Join(e.project, "node_modules", ".bin")
	assert.NilError(t, os.MkdirAll(pkg, 0o750))
	assert.NilError(t, os.MkdirAll(bin, 0o750))
	assert.NilError(t, os.WriteFile(filepath.Join(pkg, "package.json"), []byte(`{"version": "`+version+`"}`), 0o600))
	assert.NilError(t, os.WriteFile(filepath.Join(bin, "cypress"), []byte(fakeCypress), 0o700)) //nolint:gosec // an executable script
	circleci := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(circleci, "circleci"), []byte(fakeTestsuite), 0o700)) //nolint:gosec // an executable script
	e.vars = map[string]string{
		"CYPRESS_CACHE_FOLDER": e.folder,
		"FAKE_CYPRESS_VERSION": version,
		"FAKE_CYPRESS_LOG":     e.log,
		"FAKE_TESTSUITE_LOG":   e.testsuite,
		"PATH":                 circleci + string(os.PathListSeparator) + os.Getenv("PATH"),
	}
	return e
}

func (e *env) run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	vars := e.sandbox.Env()
	maps.Copy(vars, e.vars)
	r := runner.New(t, binaryPath, runner.WithSandboxEnv(vars))
	_, err := r.Run(args...)
	return r.StdoutString() + r.StderrString(), err
}

// calls returns the fake Cypress CLI's log, one call per line, and empties it.
func (e *env) calls(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(e.log)
	if os.IsNotExist(err) {
		return nil
	}
	assert.NilError(t, err)
	assert.NilError(t, os.Remove(e.log))
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

// testsuiteRun returns what the fake testsuite was run with, or nil if it was
// not run.
func (e *env) testsuiteRun(t *testing.T) map[string]string {
	t.Helper()
	b, err := os.ReadFile(e.testsuite)
	if os.IsNotExist(err) {
		return nil
	}
	assert.NilError(t, err)
	got := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		k, v, _ := strings.Cut(line, "=")
		got[k] = v
	}
	return got
}

// artifacts returns the stored artifacts, destination to path.
func (e *env) artifacts() map[string]string {
	got := map[string]string{}
	for _, a := range e.sandbox.Cache.StoredArtifacts() {
		got[a.Destination] = a.Path
	}
	return got
}

func (e *env) exported(t *testing.T) map[string]string {
	t.Helper()
	vars, err := bashenv.Vars(e.sandbox.BashEnv, map[string]string{})
	assert.NilError(t, err)
	return vars
}
