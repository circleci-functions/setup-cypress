package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/circleci-functions/toolkit/sdk"

	"github.com/circleci-functions/setup-cypress/cypress"
)

type runCmd struct {
	WorkingDirectory  string `name:"working-directory" default:"." help:"Directory holding the project's package.json, with its dependencies installed. testsuite runs here." example:"apps/web"`
	Suite             string `required:"" help:"Test suite to run with circleci testsuite. Unless .circleci/test-suites.yml defines it, testsuite's default Cypress suite runs: the specs under cypress/e2e, reported with cypress-circleci-reporter." example:"e2e"`
	ScreenshotsFolder string `name:"screenshots-folder" default:"cypress/screenshots" help:"Where Cypress saves screenshots, as its screenshotsFolder setting, relative to the working directory. Stored as artifacts after the run."`
	VideosFolder      string `name:"videos-folder" default:"cypress/videos" help:"Where Cypress saves videos, as its videosFolder setting, relative to the working directory. Stored as artifacts after the run."`
	BaseURL           string `name:"base-url" help:"URL of the site to test, passed to Cypress as baseUrl. Empty uses DEPLOYMENT_URL, then VERCEL_DEPLOYMENT_URL, which a deploy step exports, and with neither set keeps the Cypress config's baseUrl." example:"https://my-app.example.com"`
}

func (c *runCmd) Run(ctx context.Context, client *sdk.Client) error {
	testsuite, err := exec.LookPath("circleci")
	if err != nil {
		return errors.New("the circleci CLI is not on PATH: run this step in a CircleCI job, where it is installed with testsuite")
	}
	p, err := install(ctx, client, c.WorkingDirectory)
	if err != nil {
		return err
	}

	env := append(cypress.Env(os.Environ()), cacheFolderEnvVar+"="+p.CacheFolder)
	if base := baseURL(c.BaseURL); base != "" {
		env = append(env, "CYPRESS_BASE_URL="+base)
		fmt.Fprintf(os.Stderr, "setup-cypress: testing %s\n", base)
	}

	cmd := exec.CommandContext(ctx, testsuite, "testsuite", "run", c.Suite, "--runner=cypress") //nolint:gosec // the job's own CLI
	cmd.Dir = p.Dir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Cancelling the step stops testsuite gently first, so it can stop Cypress
	// and report what ran.
	cmd.Cancel = func() error { return interrupt(cmd.Process) }
	cmd.WaitDelay = 20 * time.Second

	err = cmd.Run()
	c.storeMedia(ctx, client, p.Dir)
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("circleci testsuite run %q failed (exit %d)", c.Suite, exitErr.ExitCode())
	}
	return err
}

// storeMedia stores the screenshots and videos Cypress saved, whether or not
// the suite passed: by default Cypress takes screenshots only of failures.
// The step may have been cancelled, and what was saved is still worth keeping.
func (c *runCmd) storeMedia(ctx context.Context, client *sdk.Client, dir string) {
	ctx = context.WithoutCancel(ctx)
	store(ctx, client, inDir(dir, c.ScreenshotsFolder), "cypress/screenshots")
	store(ctx, client, inDir(dir, c.VideosFolder), "cypress/videos")
}

// store keeps a directory as artifacts, if it has anything in it. Outside a
// job, or if the store fails, it says so and carries on: a failed store
// should not hide the suite's own result.
func store(ctx context.Context, client *sdk.Client, path, destination string) {
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) == 0 {
		return
	}
	err = client.StoreArtifacts(ctx, path, destination)
	if err != nil && !errors.Is(err, sdk.ErrNoServer) {
		fmt.Fprintf(os.Stderr, "setup-cypress: could not store %s: %v\n", destination, err)
	}
}

// inDir resolves path against dir, as Cypress resolves its folders against
// the project.
func inDir(dir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(dir, path)
}

// deploymentURLEnvVars are where deploy steps leave the URL they deployed to,
// in the order they are tried: the name deploy functions share, then the one
// setup-vercel exports.
var deploymentURLEnvVars = []string{"DEPLOYMENT_URL", "VERCEL_DEPLOYMENT_URL"}

// baseURL returns the site to test, with a scheme, or "" to leave Cypress's
// own config in charge.
func baseURL(flag string) string {
	base := strings.TrimSpace(flag)
	for _, name := range deploymentURLEnvVars {
		if base != "" {
			break
		}
		base = strings.TrimSpace(os.Getenv(name))
	}
	if base == "" {
		return ""
	}
	if !strings.Contains(base, "://") {
		base = "https://" + base
	}
	return strings.TrimRight(base, "/")
}

func interrupt(p *os.Process) error {
	if runtime.GOOS == "windows" {
		return p.Kill()
	}
	return p.Signal(syscall.SIGINT)
}
