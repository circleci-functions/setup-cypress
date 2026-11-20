//go:build acceptance

// Package acceptance_test runs setup-cypress against the real Cypress npm
// package and binary, downloaded from npm and Cypress's CDN. Tagged out of
// `task test` because it needs Node.js, the network and, on Linux, the
// libraries and Xvfb that Cypress needs to verify its binary.
package acceptance_test

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/circleci-functions/toolkit/bashenv"
	"github.com/circleci-functions/toolkit/sdk/sdktest"
	"github.com/circleci-functions/toolkit/testing/compiler"
	"github.com/circleci-functions/toolkit/testing/runner"
)

// binaryPathEnvVar names a prebuilt setup-cypress to test, for an executor
// with no Go toolchain to compile one with.
const binaryPathEnvVar = "SETUP_CYPRESS_BINARY"

// version is the Cypress the test project pins.
const version = "16.3.0"

// The test project is embedded, so the cross-compiled suite carries it to the
// smoke jobs, which have no checkout.
//
//go:embed testdata/project
var testdata embed.FS

var binaryPath string

// sharedFolder is a Cypress cache folder the run tests share, so that they
// download the binary once between them.
var sharedFolder string

func TestMain(m *testing.M) {
	status, err := runTests(m)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	os.Exit(status)
}

func runTests(m *testing.M) (int, error) {
	dir, err := os.MkdirTemp("", "setup-cypress-acceptance-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	sharedFolder = filepath.Join(dir, "Cypress")

	if bin := os.Getenv(binaryPathEnvVar); bin != "" {
		abs, err := filepath.Abs(bin)
		if err != nil {
			return 0, fmt.Errorf("%s=%q: %w", binaryPathEnvVar, bin, err)
		}
		if _, err := os.Stat(abs); err != nil {
			return 0, fmt.Errorf("%s=%q: %w", binaryPathEnvVar, bin, err)
		}
		binaryPath = abs
		return m.Run(), nil
	}

	p := compiler.NewParallel(1)
	defer p.Cleanup()
	p.Add(compiler.Work{Result: &binaryPath, Name: "setup-cypress", Target: "..", Source: "."})
	if err := p.Run(context.Background()); err != nil {
		return 0, err
	}
	return m.Run(), nil
}

// env is a test's sandbox, the test project with its dependencies installed,
// and the variables a job would set.
type env struct {
	sandbox *sdktest.Sandbox
	project string
	folder  string
	vars    map[string]string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{
		sandbox: sdktest.NewSandbox(t, sdktest.WithLiveCache()),
		project: project(t),
		folder:  filepath.Join(t.TempDir(), "Cypress"),
	}
	e.vars = map[string]string{"CYPRESS_CACHE_FOLDER": e.folder}
	if runtime.GOOS == "windows" {
		for _, name := range windowsProfileEnvVars {
			e.vars[name] = os.Getenv(name)
		}
	}
	return e
}

// windowsProfileEnvVars are the variables Windows gives every process, which
// a sandbox does not pass on and Cypress fails to start without.
var windowsProfileEnvVars = []string{
	"APPDATA", "LOCALAPPDATA", "USERPROFILE", "SystemRoot", "SystemDrive",
	"windir", "TEMP", "TMP", "ComSpec", "PATHEXT", "ProgramData", "ProgramFiles",
}

func (e *env) run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	vars := e.sandbox.Env()
	maps.Copy(vars, e.vars)
	r := runner.New(t, binaryPath, runner.WithSandboxEnv(vars))
	_, err := r.Run(args...)
	return r.StdoutString() + r.StderrString(), err
}

func (e *env) exported(t *testing.T) map[string]string {
	t.Helper()
	vars, err := bashenv.Vars(e.sandbox.BashEnv, map[string]string{})
	assert.NilError(t, err)
	return vars
}

// project copies the test project and installs its dependencies the way a job
// is told to: with the binary download skipped, for setup-cypress to do.
func project(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sub, err := fs.Sub(testdata, "testdata/project")
	assert.NilError(t, err)
	assert.NilError(t, os.CopyFS(dir, sub))
	cmd := exec.Command(npm(), "install", "--no-audit", "--no-fund", "--loglevel=error")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CYPRESS_INSTALL_BINARY=0")
	out, err := cmd.CombinedOutput()
	assert.NilError(t, err, string(out))
	return dir
}

// cypressCLI returns the test project's Cypress CLI, to run as a later step.
func cypressCLI(dir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, "node_modules", ".bin", "cypress.cmd")
	}
	return filepath.Join(dir, "node_modules", ".bin", "cypress")
}

func npm() string {
	if runtime.GOOS == "windows" {
		return "npm.cmd"
	}
	return "npm"
}
