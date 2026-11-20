package clitest_test

import (
	"maps"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestRun_InstallsThenRunsTheSuite(t *testing.T) {
	e := newEnv(t)
	e.vars["CYPRESS_INSTALL_BINARY"] = "0"

	out, err := e.run(t, "run", "--suite=e2e", "--working-directory="+e.project)
	assert.NilError(t, err, out)
	assert.Check(t, cmp.Contains(out, "Cypress "+version+" downloaded"))
	project, err := filepath.EvalSymlinks(e.project)
	assert.NilError(t, err)
	assert.Check(t, cmp.DeepEqual(e.testsuiteRun(t), map[string]string{
		"args":   "testsuite run e2e --runner=cypress",
		"dir":    project,
		"base":   "",
		"folder": e.folder,
		"skip":   "",
	}))
}

func TestRun_IsTheDefaultCommand(t *testing.T) {
	e := newEnv(t)

	out, err := e.run(t, "--suite=e2e", "--working-directory="+e.project)
	assert.NilError(t, err, out)
	assert.Check(t, cmp.Equal(e.testsuiteRun(t)["args"], "testsuite run e2e --runner=cypress"))
}

func TestRun_FindsTheBinaryWhereCypressDefaultsTo(t *testing.T) {
	e := newEnv(t)
	delete(e.vars, "CYPRESS_CACHE_FOLDER")

	out, err := e.run(t, "--suite=e2e", "--working-directory="+e.project)
	assert.NilError(t, err, out)
	assert.Check(t, cmp.Equal(e.testsuiteRun(t)["folder"], filepath.Join(e.sandbox.Home, ".cache", "Cypress")))
}

func TestRun_PassesTheBaseURL(t *testing.T) {
	tests := map[string]string{
		"https://app.example.com":   "https://app.example.com",
		"https://app.example.com/":  "https://app.example.com",
		"app.example.com":           "https://app.example.com",
		"http://localhost:3000/app": "http://localhost:3000/app",
	}
	for flag, want := range tests {
		t.Run(flag, func(t *testing.T) {
			e := newEnv(t)

			out, err := e.run(t, "--suite=e2e", "--working-directory="+e.project, "--base-url="+flag)
			assert.NilError(t, err, out)
			assert.Check(t, cmp.Equal(e.testsuiteRun(t)["base"], want))
			assert.Check(t, cmp.Contains(out, "testing "+want))
		})
	}
}

func TestRun_FailsWhenTheSuiteFails(t *testing.T) {
	e := newEnv(t)
	e.vars["FAKE_TESTSUITE_EXIT"] = "3"

	out, err := e.run(t, "--suite=e2e", "--working-directory="+e.project)
	assert.Check(t, err != nil, out)
	assert.Check(t, cmp.Contains(out, `circleci testsuite run "e2e" failed (exit 3)`))
}

func TestRun_NeedsASuite(t *testing.T) {
	e := newEnv(t)

	out, err := e.run(t, "--working-directory="+e.project)
	assert.Check(t, err != nil, out)
	assert.Check(t, cmp.Contains(out, "--suite"))
	assert.Check(t, cmp.Nil(e.testsuiteRun(t)))
}

func TestRun_NeedsTheCircleCICLI(t *testing.T) {
	e := newEnv(t)
	e.vars["PATH"] = t.TempDir()

	out, err := e.run(t, "--suite=e2e", "--working-directory="+e.project)
	assert.Check(t, err != nil, out)
	assert.Check(t, cmp.Contains(out, "the circleci CLI is not on PATH"))
	_, statErr := os.Stat(e.folder)
	assert.Check(t, os.IsNotExist(statErr), "nothing should be installed")
}

func TestRun_DoesNotRunTheSuiteWhenInstallFails(t *testing.T) {
	e := newEnv(t)
	e.vars["FAKE_CYPRESS_VERIFY_FAILS"] = "1"

	out, err := e.run(t, "--suite=e2e", "--working-directory="+e.project)
	assert.Check(t, err != nil, out)
	assert.Check(t, cmp.Nil(e.testsuiteRun(t)))
}

func TestRun_DefaultsToTheDeployment(t *testing.T) {
	tests := []struct {
		name string
		vars map[string]string
		flag string
		want string
	}{
		{"deployment", map[string]string{"DEPLOYMENT_URL": "https://deploy.example.com"}, "", "https://deploy.example.com"},
		{"vercel deployment", map[string]string{"VERCEL_DEPLOYMENT_URL": "my-app-abc123.vercel.app"}, "", "https://my-app-abc123.vercel.app"},
		{"deployment first", map[string]string{"DEPLOYMENT_URL": "https://deploy.example.com", "VERCEL_DEPLOYMENT_URL": "my-app.vercel.app"}, "", "https://deploy.example.com"},
		{"flag first", map[string]string{"DEPLOYMENT_URL": "https://deploy.example.com"}, "https://flag.example.com", "https://flag.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			maps.Copy(e.vars, tt.vars)

			args := []string{"--suite=e2e", "--working-directory=" + e.project}
			if tt.flag != "" {
				args = append(args, "--base-url="+tt.flag)
			}
			out, err := e.run(t, args...)
			assert.NilError(t, err, out)
			assert.Check(t, cmp.Equal(e.testsuiteRun(t)["base"], tt.want))
		})
	}
}
