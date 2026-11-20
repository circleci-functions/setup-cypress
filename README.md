# setup-cypress

A CircleCI function that installs the [Cypress](https://www.cypress.io) binary your project needs,
caching it across jobs.

```yaml
functions:
  node: github.com/circleci-functions/setup-node@v0.5.0
  cypress: github.com/circleci-functions/setup-cypress@v0.1.0

jobs:
  e2e:
    docker:
      - image: cimg/node:lts-browsers
    environment:
      CYPRESS_INSTALL_BINARY: "0"     # leave the binary to setup-cypress
    steps:
      - checkout
      - node/install-packages
      - cypress/install
```

Run `circleci function add github.com/circleci-functions/setup-cypress --as cypress` to write the
entry.

## install

Installs and verifies the binary that the project's `cypress` package needs, and exports
`CYPRESS_CACHE_FOLDER` so later steps find it.

| Flag | Default | Description |
|---|---|---|
| `working-directory` | `.` | Directory holding the project's `package.json`, with its dependencies installed |

It uses the project's own Cypress from `node_modules`, so install your dependencies first. Set
`CYPRESS_INSTALL_BINARY: "0"` on the job so that install skips the binary download, which
setup-cypress then restores from the cache. setup-cypress ignores that setting for its own
download; any other value, such as a mirror's URL, is passed on to Cypress.

### Caching

The binary is cached under its Cypress version, the platform and the cache folder. A change to
other dependencies keeps the cached binary, and upgrading Cypress caches the new version once.
Only the version in use is saved, so the entry does not grow with old versions. The cache folder
is Cypress's own: `CYPRESS_CACHE_FOLDER` if set, otherwise the platform default.

## Platforms

Linux, macOS and Windows, on amd64 and arm64. On Linux, Cypress needs
[its system dependencies](https://on.cypress.io/required-dependencies) and Xvfb, which the
`cimg/node:*-browsers` images have.

## Releasing

See [RELEASE.md](RELEASE.md).
