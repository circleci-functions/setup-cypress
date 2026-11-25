package clitest_test

import (
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestRun_StoresScreenshotsAndVideosOfAFailedRun(t *testing.T) {
	e := newEnv(t)
	e.vars["FAKE_TESTSUITE_MEDIA"] = "cypress/screenshots cypress/videos"
	e.vars["FAKE_TESTSUITE_EXIT"] = "1"

	out, err := e.run(t, "--suite=e2e", "--working-directory="+e.project)
	assert.Check(t, err != nil, out)
	assert.Check(t, cmp.DeepEqual(e.artifacts(), map[string]string{
		"cypress/screenshots": filepath.Join(e.project, "cypress", "screenshots"),
		"cypress/videos":      filepath.Join(e.project, "cypress", "videos"),
	}))
}

func TestRun_StoresMediaFromTheConfiguredFolders(t *testing.T) {
	e := newEnv(t)
	videos := filepath.Join(t.TempDir(), "videos")
	e.vars["FAKE_TESTSUITE_MEDIA"] = "e2e/shots " + videos

	out, err := e.run(t, "--suite=e2e", "--working-directory="+e.project,
		"--screenshots-folder=e2e/shots", "--videos-folder="+videos)
	assert.NilError(t, err, out)
	assert.Check(t, cmp.DeepEqual(e.artifacts(), map[string]string{
		"cypress/screenshots": filepath.Join(e.project, "e2e", "shots"),
		"cypress/videos":      videos,
	}))
}

func TestRun_StoresNothingWhenCypressSavedNothing(t *testing.T) {
	e := newEnv(t)

	out, err := e.run(t, "--suite=e2e", "--working-directory="+e.project)
	assert.NilError(t, err, out)
	assert.Check(t, cmp.Len(e.artifacts(), 0))
}
