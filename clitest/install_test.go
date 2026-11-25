package clitest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestInstall_DownloadsAndCachesTheBinary(t *testing.T) {
	e := newEnv(t)

	out, err := e.run(t, "install", "--working-directory="+e.project)
	assert.NilError(t, err, out)
	assert.Check(t, cmp.Contains(out, "Cypress "+version+" downloaded"))
	assert.Check(t, cmp.DeepEqual(e.calls(t), []string{
		"cache path absent skip=",
		"install absent skip=",
		"verify present skip=",
	}))
	keys := e.sandbox.Cache.SavedKeys()
	assert.Assert(t, cmp.Len(keys, 1))
	assert.Check(t, strings.HasPrefix(keys[0], "setup-cypress-binary-v1-"), keys[0])
	assert.Check(t, cmp.Contains(keys[0], "-"+version+"-"))
	assert.Check(t, cmp.Equal(e.exported(t)["CYPRESS_CACHE_FOLDER"], e.folder))
}

func TestInstall_RestoresTheBinaryFromTheCache(t *testing.T) {
	e := newEnv(t)
	out, err := e.run(t, "install", "--working-directory="+e.project)
	assert.NilError(t, err, out)
	e.calls(t)
	assert.NilError(t, os.RemoveAll(e.folder))

	out, err = e.run(t, "install", "--working-directory="+e.project)
	assert.NilError(t, err, out)
	assert.Check(t, cmp.Contains(out, "Cypress "+version+" restored from the cache"))
	assert.Check(t, cmp.DeepEqual(e.calls(t), []string{
		"cache path absent skip=",
		"install present skip=",
		"verify present skip=",
	}))
	assert.Check(t, cmp.Len(e.sandbox.Cache.SavedKeys(), 1))
}

func TestInstall_LeavesAnInstalledBinaryAlone(t *testing.T) {
	e := newEnv(t)
	assert.NilError(t, os.MkdirAll(filepath.Join(e.folder, version, "Cypress"), 0o750))

	out, err := e.run(t, "install", "--working-directory="+e.project)
	assert.NilError(t, err, out)
	assert.Check(t, cmp.Contains(out, "Cypress "+version+" already installed"))
	assert.Check(t, cmp.Len(e.sandbox.Cache.SavedKeys(), 0))
}

func TestInstall_DownloadsDespiteTheJobSkippingIt(t *testing.T) {
	e := newEnv(t)
	e.vars["CYPRESS_INSTALL_BINARY"] = "0"

	out, err := e.run(t, "install", "--working-directory="+e.project)
	assert.NilError(t, err, out)
	assert.Check(t, cmp.DeepEqual(e.calls(t), []string{
		"cache path absent skip=",
		"install absent skip=",
		"verify present skip=",
	}))
}

func TestInstall_KeysTheCacheByFolder(t *testing.T) {
	e := newEnv(t)
	out, err := e.run(t, "install", "--working-directory="+e.project)
	assert.NilError(t, err, out)

	e.vars["CYPRESS_CACHE_FOLDER"] = filepath.Join(t.TempDir(), "Cypress")
	out, err = e.run(t, "install", "--working-directory="+e.project)
	assert.NilError(t, err, out)
	assert.Check(t, cmp.Contains(out, "downloaded"))
	assert.Check(t, cmp.Len(e.sandbox.Cache.SavedKeys(), 2))
}

func TestInstall_FailsWhenTheBinaryDoesNotVerify(t *testing.T) {
	e := newEnv(t)
	e.vars["FAKE_CYPRESS_VERIFY_FAILS"] = "1"

	out, err := e.run(t, "install", "--working-directory="+e.project)
	assert.Check(t, err != nil, out)
	assert.Check(t, cmp.Contains(out, "cypress verify:"))
	assert.Check(t, cmp.Len(e.sandbox.Cache.SavedKeys(), 0))
}

func TestInstall_NeedsTheProjectsDependencies(t *testing.T) {
	e := newEnv(t)
	dir := t.TempDir()

	out, err := e.run(t, "install", "--working-directory="+dir)
	assert.Check(t, err != nil, out)
	assert.Check(t, cmp.Contains(out, "cypress is not installed in "+dir))
}
