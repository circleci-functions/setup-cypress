package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/circleci-functions/toolkit/bashenv"
	"github.com/circleci-functions/toolkit/sdk"

	"github.com/circleci-functions/setup-cypress/cypress"
)

// cacheFolderEnvVar is where Cypress looks for its binaries. It is exported so
// later steps find the binary this step installed.
const cacheFolderEnvVar = "CYPRESS_CACHE_FOLDER"

type installCmd struct {
	WorkingDirectory string `name:"working-directory" default:"." help:"Directory holding the project's package.json, with its dependencies installed." example:"apps/web"`
}

func (c *installCmd) Run(ctx context.Context, client *sdk.Client) error {
	_, err := install(ctx, client, c.WorkingDirectory)
	return err
}

// installed is a project whose Cypress binary is ready to run.
type installed struct {
	*cypress.Project
	// CacheFolder is where the binary is, which Cypress must be told when the
	// folder came from this step's export rather than the environment.
	CacheFolder string
}

// install makes sure the binary for the Cypress installed in dir is in
// Cypress's cache folder, restoring it from the job cache or downloading it,
// verifies it, and tells later steps where it is. `cypress install` runs even
// after a restore, since it finishes or replaces a binary that is incomplete
// and does nothing to one that is not.
func install(ctx context.Context, client *sdk.Client, dir string) (*installed, error) {
	p, err := cypress.Open(dir)
	if err != nil {
		return nil, err
	}
	folder, err := p.CacheFolder(ctx)
	if err != nil {
		return nil, err
	}
	binary := filepath.Join(folder, p.Version)
	key := binaryCacheKey(p.Version, folder)

	source := "already installed"
	if !exists(binary) {
		_, err := client.RestoreCache(ctx, []string{key}, sdk.WithStepName("Restoring the Cypress binary"))
		sdk.LogCacheErr("restore", err)
		source = "restored from the cache"
	}
	downloaded := !exists(binary)
	if downloaded {
		source = "downloaded"
	}
	if err := p.Install(ctx); err != nil {
		return nil, err
	}
	if err := p.Verify(ctx); err != nil {
		return nil, err
	}
	if downloaded {
		sdk.LogCacheErr("save", client.SaveCache(ctx, key, []string{binary}, sdk.WithStepName("Saving the Cypress binary")))
	}

	fmt.Fprintf(os.Stderr, "setup-cypress: Cypress %s %s, in %s\n", p.Version, source, binary)
	if err := export(map[string]string{cacheFolderEnvVar: folder}); err != nil {
		return nil, err
	}
	return &installed{Project: p, CacheFolder: folder}, nil
}

// binaryCacheKey names the cache entry for one Cypress version on one
// platform. A cache restores to the absolute path it was saved from, so the
// cache folder is part of the key: an entry saved from another folder would
// restore where this job's Cypress does not look.
func binaryCacheKey(version, folder string) string {
	sum := sha256.Sum256([]byte(folder))
	return fmt.Sprintf("setup-cypress-binary-v1-%s-%s-%s", sdk.OSArch(), version, hex.EncodeToString(sum[:4]))
}

// export passes values to later steps. Outside a job there is nowhere to put them.
func export(vars map[string]string) error {
	err := bashenv.ExportEnvVars(vars)
	if errors.Is(err, bashenv.ErrNoBashEnv) {
		return nil
	}
	return err
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}
