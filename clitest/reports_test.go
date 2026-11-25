package clitest_test

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestRun_StoresTheJUnitReports(t *testing.T) {
	e := newEnv(t)
	e.vars["FAKE_TESTSUITE_REPORTS"] = "123"
	e.vars["FAKE_TESTSUITE_EXIT"] = "1"

	out, err := e.run(t, "--suite=e2e", "--working-directory="+e.project)
	assert.Check(t, err != nil, out)
	assert.Check(t, cmp.DeepEqual(e.artifacts(), map[string]string{
		"cypress/results": filepath.Join(e.tmp, "test-suite-outputs.junit-directory-123"),
	}))
}

func TestRun_StoresEachJUnitOutput(t *testing.T) {
	e := newEnv(t)
	e.vars["FAKE_TESTSUITE_REPORTS"] = "1 2"

	out, err := e.run(t, "--suite=e2e", "--working-directory="+e.project)
	assert.NilError(t, err, out)
	assert.Check(t, cmp.DeepEqual(e.artifacts(), map[string]string{
		"cypress/results/1": filepath.Join(e.tmp, "test-suite-outputs.junit-directory-1"),
		"cypress/results/2": filepath.Join(e.tmp, "test-suite-outputs.junit-directory-2"),
	}))
}

func TestRun_LeavesEarlierRunsReportsAlone(t *testing.T) {
	e := newEnv(t)
	earlier := filepath.Join(e.tmp, "test-suite-outputs.junit-directory-1", "1")
	assert.NilError(t, os.MkdirAll(earlier, 0o750))
	assert.NilError(t, os.WriteFile(filepath.Join(earlier, "result-1.xml"), []byte("<testsuites/>"), 0o600))
	e.vars["FAKE_TESTSUITE_REPORTS"] = "2"

	out, err := e.run(t, "--suite=e2e", "--working-directory="+e.project)
	assert.NilError(t, err, out)
	assert.Check(t, cmp.DeepEqual(e.artifacts(), map[string]string{
		"cypress/results": filepath.Join(e.tmp, "test-suite-outputs.junit-directory-2"),
	}))
}
