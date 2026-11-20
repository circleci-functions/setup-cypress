package cypress_test

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/circleci-functions/setup-cypress/cypress"
)

func writePackage(t *testing.T, dir, manifest string) {
	t.Helper()
	pkg := filepath.Join(dir, "node_modules", "cypress")
	assert.NilError(t, os.MkdirAll(pkg, 0o750))
	assert.NilError(t, os.WriteFile(filepath.Join(pkg, "package.json"), []byte(manifest), 0o600))
}

func TestOpen_ReadsTheInstalledVersion(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, `{"name": "cypress", "version": "15.7.1"}`)

	p, err := cypress.Open(dir)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(p.Version, "15.7.1"))
	assert.Check(t, cmp.Equal(p.Dir, dir))
}

func TestOpen_NeedsTheProjectsDependencies(t *testing.T) {
	dir := t.TempDir()

	_, err := cypress.Open(dir)
	assert.Check(t, cmp.ErrorContains(err, "cypress is not installed in "+dir))
	assert.Check(t, cmp.ErrorContains(err, "before this step"))
}

func TestOpen_RejectsABrokenManifest(t *testing.T) {
	tests := map[string]string{
		"not JSON":   `{`,
		"no version": `{"name": "cypress"}`,
	}
	for name, manifest := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writePackage(t, dir, manifest)

			_, err := cypress.Open(dir)
			assert.Check(t, cmp.ErrorContains(err, filepath.Join(dir, "node_modules", "cypress", "package.json")))
		})
	}
}

func TestEnv_DropsOnlyTheDownloadSkip(t *testing.T) {
	got := cypress.Env([]string{
		"PATH=/bin",
		"CYPRESS_INSTALL_BINARY=0",
		"CYPRESS_CACHE_FOLDER=/cache",
	})
	assert.Check(t, cmp.DeepEqual(got, []string{"PATH=/bin", "CYPRESS_CACHE_FOLDER=/cache"}))
}

func TestEnv_KeepsAMirror(t *testing.T) {
	env := []string{"CYPRESS_INSTALL_BINARY=https://mirror.example.com/cypress.zip"}
	assert.Check(t, cmp.DeepEqual(cypress.Env(env), env))
}
