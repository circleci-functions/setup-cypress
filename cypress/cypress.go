// Package cypress drives the Cypress a project has installed in its
// node_modules: its version, where it keeps its binary, and installing and
// verifying that binary.
package cypress

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// installBinaryEnvVar is the variable Cypress's npm package reads to skip or
// redirect the binary download on install.
const installBinaryEnvVar = "CYPRESS_INSTALL_BINARY"

// Project is a directory with Cypress installed in its node_modules.
type Project struct {
	// Dir is the project's absolute path.
	Dir string
	// Version is the installed Cypress package's version, which is also the
	// version of the binary it needs.
	Version string
	// Stdout and Stderr receive the output of the Cypress commands run.
	Stdout, Stderr io.Writer
}

// Open returns the project in dir. It fails if Cypress is not installed there,
// since the binary's version and the commands that install it both come from
// the npm package.
func Open(dir string) (*Project, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	manifest := filepath.Join(abs, "node_modules", "cypress", "package.json")
	b, err := os.ReadFile(manifest) //nolint:gosec // the project's own package
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("cypress is not installed in %s: install the project's dependencies (npm ci, pnpm install, yarn install) before this step", abs)
	}
	if err != nil {
		return nil, err
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(b, &pkg); err != nil {
		return nil, fmt.Errorf("%s: %w", manifest, err)
	}
	if pkg.Version == "" {
		return nil, fmt.Errorf("%s has no version", manifest)
	}
	return &Project{Dir: abs, Version: pkg.Version, Stdout: os.Stdout, Stderr: os.Stderr}, nil
}

// CacheFolder returns the folder Cypress keeps its binaries in, one
// subdirectory per version. Cypress works it out from CYPRESS_CACHE_FOLDER and
// the platform, so it is asked rather than second-guessed.
func (p *Project) CacheFolder(ctx context.Context) (string, error) {
	var out bytes.Buffer
	cmd := p.command(ctx, "cache", "path")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("cypress cache path: %w", err)
	}
	dir := strings.TrimSpace(out.String())
	if dir == "" {
		return "", errors.New("cypress cache path printed nothing")
	}
	return dir, nil
}

// Install downloads the binary for this project's version, if it is not in the
// cache folder already.
func (p *Project) Install(ctx context.Context) error {
	if err := p.command(ctx, "install").Run(); err != nil {
		return fmt.Errorf("cypress install: %w", err)
	}
	return nil
}

// Verify checks the installed binary runs on this machine.
func (p *Project) Verify(ctx context.Context) error {
	if err := p.command(ctx, "verify").Run(); err != nil {
		return fmt.Errorf("cypress verify: %w", err)
	}
	return nil
}

// command runs the project's own Cypress CLI, never one npx would download.
func (p *Project) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, p.bin(), args...) //nolint:gosec // the project's own Cypress
	cmd.Dir = p.Dir
	cmd.Env = Env(os.Environ())
	cmd.Stdout = p.Stdout
	cmd.Stderr = p.Stderr
	return cmd
}

func (p *Project) bin() string {
	name := "cypress"
	if runtime.GOOS == "windows" {
		name += ".cmd"
	}
	return filepath.Join(p.Dir, "node_modules", ".bin", name)
}

// Env returns env for running Cypress. A job sets CYPRESS_INSTALL_BINARY=0 so
// its dependency install skips the download that this function caches, so
// that setting is dropped here, or `cypress install` would skip it too. Any
// other value, such as a mirror's URL, is the user's choice and kept.
func Env(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if name, value, _ := strings.Cut(kv, "="); name == installBinaryEnvVar && value == "0" {
			continue
		}
		out = append(out, kv)
	}
	return out
}
