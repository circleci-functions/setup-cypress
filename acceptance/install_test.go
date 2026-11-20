//go:build acceptance

package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/circleci-functions/toolkit/sdk/sdktest"
	"github.com/circleci-functions/toolkit/testing/runner"
)

func TestInstall_DownloadsVerifiesAndRestores(t *testing.T) {
	e := newEnv(t)

	out, err := e.run(t, "install", "--working-directory="+e.project)
	assert.NilError(t, err, out)
	assert.Check(t, cmp.Contains(out, "Cypress "+version+" downloaded"))
	assert.Check(t, cmp.Equal(e.exported(t)["CYPRESS_CACHE_FOLDER"], e.folder))

	cmd := exec.Command(cypressCLI(e.project), "version", "--component", "binary")
	cmd.Dir = e.project
	cmd.Env = append(os.Environ(), "CYPRESS_CACHE_FOLDER="+e.folder)
	got, err := cmd.CombinedOutput()
	assert.NilError(t, err, string(got))
	assert.Check(t, cmp.Contains(string(got), version))

	if runtime.GOOS == "darwin" && !e.sandbox.LiveCache() {
		t.Skip("the in-memory cache cannot save the symlinks in Cypress.app; set TOOLKIT_CACHE_MODE=live in a job to test the restore")
	}
	assert.NilError(t, os.RemoveAll(e.folder))
	out, err = e.run(t, "install", "--working-directory="+e.project)
	assert.NilError(t, err, out)
	assert.Check(t, cmp.Contains(out, "Cypress "+version+" restored from the cache"))
}

// TestInstall_UsesCypresssOwnCacheFolder is the one test that lets Cypress
// choose its cache folder, rather than being told with CYPRESS_CACHE_FOLDER.
// That choice depends on the executor's platform and home directory, which a
// sandbox replaces, so it runs only where the machine is disposable.
func TestInstall_UsesCypresssOwnCacheFolder(t *testing.T) {
	sdktest.SkipUnlessInJob(t)
	dir := project(t)

	bashEnv := filepath.Join(t.TempDir(), "bash_env")
	r := runner.New(t, binaryPath,
		runner.WithUnsafeOSEnvVars(),
		runner.WithUnsafeAdditionalEnvVars(map[string]string{
			"BASH_ENV":             bashEnv,
			"CYPRESS_CACHE_FOLDER": "",
		}))
	_, err := r.Run("install", "--working-directory="+dir)
	out := r.StdoutString() + r.StderrString()
	assert.NilError(t, err, out)
	t.Logf("setup-cypress reported: %s", out)

	folder, err := sdktest.LaterStep(t, cypressCLI(dir), "cache", "path")
	assert.NilError(t, err, folder)
	assert.Check(t, cmp.Contains(out, filepath.Join(strings.TrimSpace(folder), version)))
}
