# setup-cypress

A CircleCI function that installs the [Cypress](https://www.cypress.io) binary your project needs,
caching it across jobs, and runs your Cypress suite with `circleci testsuite`, which splits it
across parallel containers and stores the results.

```yaml
functions:
  node: github.com/circleci-functions/setup-node@v0.5.0
  cypress: github.com/circleci-functions/setup-cypress@v0.1.0

jobs:
  e2e:
    docker:
      - image: cimg/node:lts-browsers
    parallelism: 4
    environment:
      CYPRESS_INSTALL_BINARY: "0"     # leave the binary to setup-cypress
    steps:
      - checkout
      - node/install-packages
      - cypress:                      # same as cypress/run
          with:
            suite: e2e
```

Run `circleci function add github.com/circleci-functions/setup-cypress --as cypress` to write the
entry.

## run

Installs the binary as [`install`](#install) does, then runs
`circleci testsuite run <suite> --runner=cypress` in the working directory.

| Flag | Default | Description |
|---|---|---|
| `suite` | | Required. The test suite to run |
| `base-url` | `$DEPLOYMENT_URL`, then `$VERCEL_DEPLOYMENT_URL` | Site to test, passed to Cypress as `baseUrl`. With none set, the Cypress config's `baseUrl` stands |
| `working-directory` | `.` | Directory holding the project's `package.json`, with its dependencies installed |

If `.circleci/test-suites.yml` defines `suite`, testsuite runs it as written. Otherwise
`--runner=cypress` has testsuite run its default Cypress suite, which:

- finds specs with `find ./cypress/e2e -type f -name '*.cy.*'`, whatever the Cypress config's
  `specPattern` says
- runs them with [`cypress-circleci-reporter`](https://www.npmjs.com/package/cypress-circleci-reporter),
  which the project needs as a dev dependency along with `mocha`

A project that keeps its specs anywhere else, or names them differently, needs a suite of its own:
run `circleci testsuite generate <suite> --template=cypress`, change its `discover`, and commit the
file. Without one:

| Project | What happens |
|---|---|
| No `cypress/e2e` directory | The step fails: the discover command exits 1 |
| No `*.cy.*` files under `cypress/e2e` | The step fails: testsuite discovers no tests |
| Some specs under `cypress/e2e`, others elsewhere | Only those under `cypress/e2e` run, and the step can pass |
| `cypress-circleci-reporter` not installed | The step fails: Cypress cannot load the reporter |

testsuite splits the specs across the job's `parallelism` by their past timings, and stores the
results in the job's Tests tab, so the function does neither itself. The step fails if any spec
fails.

A `base-url` without a scheme gets `https://`, as deploy steps often export a bare host name.

On Windows, testsuite's default Cypress suite does not run: its `find` resolves to Windows' own
`FIND`, and it passes spec paths to bash with backslashes, which bash drops. Define the suite in
`.circleci/test-suites.yml` instead.

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
