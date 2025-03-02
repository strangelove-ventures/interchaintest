# Integrate E2E Tests Locally + in GitHub Actions

## Goal

Seamlessly build and test current iterations of your chain both locally and within GitHub's continuous integration (CI) pipeline.

### Example workflow accomplished from this guide

1. Chain code altered
2. Dev runs `make local-image` -> builds current iteration of chain inside the docker image
3. Dev runs `make test-the-thing` -> runs test using new image
4. -- Development is done, code pushed, PR created --
5. GitHub Action Builds Docker image (on PR)
6. GitHub Action runs tests as part of CI (on PR)
7. The team can ensure all tests pass before merging

### Setup

We recommend creating a separate directory in your chain's repo and importing Interchaintest as its own module. This will allow you to keep the extra imports needed for `interchaintest` separate from your chain.

See [`noble`](https://github.com/noble-assets/noble) chains `interchaintest e2e` [folder](https://github.com/noble-assets/noble/tree/main/e2e) as an example.

Nobles' CI workflow is a great example to follow along with throughout this guide.

## Building Locally

**Goal:** Dev runs `make local-image` to create a docker container of the chain's current code base. They can then use that image in the E2E tests.

`interchaintest` relies on docker images containing your chains binary.

We recommend leveraging [`heighliner`](https://github.com/strangelove-ventures/heighliner) to build the docker image for your chain. Ideally, you won't even need a local Dockerfile in your repo as Heighliner covers this.

Add this to your Makefile:

```makefile
get-heighliner:
  git clone https://github.com/strangelove-ventures/heighliner.git
  cd heighliner && go install

local-image:
ifeq (,$(shell which heighliner))
  echo 'heighliner' binary not found. Consider running `make get-heighliner`
else
  heighliner build -c noble --local --dockerfile cosmos --build-target "make install" --binaries "/go/bin/nobled"
endif
```

In the example above, `make local-image` will build image: `noble:local`.

You'll need to change `-c` arg to your chain name, and the `--binaries` arg to your binary's install location.

While Heighliner works out of the box with most Cosmos-based chains. Other chains or non-standard Cosmos chains may require extra args. See the [Heighliner](https://github.com/strangelove-ventures/heighliner) repo for more info.

It's important to realize the image will be named using the arg from the `-c` flag. The `--local` flag builds from your local repository (as opposed to the remote git repository) and tags the docker image as `local`. The image name and tag will be integrated into your tests [in the step below](#integrate-docker-name-and-tag-into-tests)

## GitHub E2E Workflow for PRs

**Goal:** When a PR is created, have a GitHub workflow that:

1. Builds a Docker image with the latest code changes to the binary.
2. Synchronously spin up runners (one for each test), load Docker image, and run tests.

We recommend having this workflow run only on PR's.

There are two jobs in this workflow.

1. `build-docker`
2. `e2e-tests`

<details>
<summary>Full workflow file example</summary>

```yaml
name: End to End Tests

on:
  pull_request:

env:
  TAR_PATH: heighliner.tar

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  build-docker:
    runs-on: ubuntu-latest
    steps:
      - name: Build Docker image
        uses: strangelove-ventures/heighliner-build-action@v0.0.3
        with:
          registry: "" # empty registry, image only shared for e2e testing
          tag: local # emulate local environment for consistency in interchaintest cases
          tar-export-path: ${{ env.TAR_PATH }} # export a tarball that can be uploaded as an artifact for the e2e jobs
          platform: linux/amd64 # test runner architecture only
          git-ref: ${{ github.head_ref }} # source code ref

          # Heighliner chains.yaml config
          chain: noble
          dockerfile: cosmos
          build-target: make install
          binaries: |
            - /go/bin/nobled

      - name: Publish Tarball as Artifact
        uses: actions/upload-artifact@4
        with:
          name: noble-docker-image
          path: ${{ env.TAR_PATH }}

  e2e-tests:
    needs: build-docker
    runs-on: ubuntu-latest
    strategy:
        matrix:
            # names of `make` commands to run tests
            test:
              - ictest-tkn-factory
              - ictest-packet-forward
              - ictest-paramauthority
              - ictest-chain-upgrade-noble-1
              - ictest-chain-upgrade-grand-1
              - ictest-globalFee
              - ictest-ics20-bps-fees
              - ictest-client-substitution
              - ictest-cctp
        fail-fast: false

    steps:
      - name: Set up Go 1.19
        uses: actions/setup-go@v3
        with:
          go-version: 1.19

      - name: checkout chain
        uses: actions/checkout@v3

      - name: Download Tarball Artifact
        uses: actions/download-artifact@v4
        with:
          name: noble-docker-image

      - name: Load Docker Image
        run: docker image load -i ${{ env.TAR_PATH }}

      - name: run test
        run: make ${{ matrix.test }}
```

</details>

This example is broken down in the steps below.

### Job 1: `build-docker`

**Goal:** Builds Docker image based on the pushed chain code. Then upload the image as a GitHub artifact.

Artifacts allow you to share data between jobs in a workflow. You can read more about them [here](https://docs.github.com/en/actions/using-workflows/storing-workflow-data-as-artifacts)

We recommend to use the [Heighliner Action](https://github.com/strangelove-ventures/heighliner-build-action) (strangelove-ventures/heighliner-build-action). This action streamlines setting up Go, checking out the repo, and building the image with the proper tags. Like the local build step above, this removes the need for a local Dockerfile.

Note the `tar-export-path`. This is the artifact that will be uploaded and used by the other GitHub runners.

```yaml
env:
  TAR_PATH: heighliner.tar

jobs:
  build-docker:
    runs-on: ubuntu-latest
    steps:
      - name: Build Docker image
        uses: strangelove-ventures/heighliner-build-action@v0.0.3
        with:
          registry: "" # empty registry, image only shared for e2e testing
          tag: local # emulate local environment for consistency in interchaintest cases
          tar-export-path: ${{ env.TAR_PATH }} # export a tarball that can be uploaded as an artifact for the e2e jobs
          platform: linux/amd64 # test runner architecture only
          git-ref: ${{ github.head_ref }} # source code ref

          # Heighliner chains.yaml config
          chain: noble
          dockerfile: cosmos
          build-target: make install
          binaries: |
            - /go/bin/nobled
```

As noted in the previous step, unless you are working with a non-Cosmos chain or a non-standard Cosmos chain, the only args you'll likely need to alter are the `chain` and `binaries` arguments.

This step uploads the artifact:

```yaml
...

env:
  TAR_PATH: heighliner.tar

jobs:
  build-docker:
...
...
      - name: Publish Tarball as Artifact
        uses: actions/upload-artifact@v4
        with:
          name: chain-docker-image
          path: ${{ env.TAR_PATH }}

```

You shouldn't need to change anything in this step.

### Job 2: `e2e-tests`

**Goal:** Synchronously spin up a runner for each test, download the artifact (Docker image) to each runner, load the image into Docker, and run the test.

The example below will spin up 9 runners, one for each test.

```yaml
  e2e-tests:
    needs: build-docker
    runs-on: ubuntu-latest
    strategy:
        matrix:
            # names of `make` commands to run tests
            test:
              - ictest-tkn-factory
              - ictest-packet-forward
              - ictest-paramauthority
              - ictest-chain-upgrade-noble-1
              - ictest-chain-upgrade-grand-1
              - ictest-globalFee
              - ictest-ics20-bps-fees
              - ictest-client-substitution
              - ictest-cctp
        fail-fast: false

    steps:
      - name: Set up Go 1.19
        uses: actions/setup-go@v3
        with:
          go-version: 1.19

      - name: checkout chain
        uses: actions/checkout@v3

      - name: Download Tarball Artifact
        uses: actions/download-artifact@v4
        with:
          name: chain-docker-image

      - name: Load Docker Image
        run: docker image load -i ${{ env.TAR_PATH }}

      - name: run test
        run: make ${{ matrix.test }}
```

The only thing you'll need to change is the `test` names in the strategy matrix. These should match `Makefile` commands.

For example in the Makefile:

```makefile
ictest-tkn-factory:
  cd interchaintest && go test -race -v -run ^TestTokenFactory$$ .
```

There are also environment variables you can set to alter logging settings. See all options [here](./envOptions.md)

## Integrate Docker Name and Tag into Tests

The final part is to specify the proper Docker `Repository` (image name) and `Version` (image tag) in your `ibc.ChainConfig`.

`Repository` = chain name (same as the `-c` arg in the heighliner command)

`Version` = "local"

```go
  cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
    {
      Name:          "noble",
      ChainConfig:   ibc.ChainConfig{
        Images: []ibc.DockerImage{
          {
            Repository: "noble",
            Version: "local",
            UidGid: "1025:1025",
          },
        },
      },
      NumValidators: &numVals,
      NumFullNodes:  &numFullNodes,
    },
  })

```

> [!NOTE]
> When running tests, you may see a `ERROR  Failed to pull image` at the start of the test. This is expected when running local images.

With that, you should be set up for streamlined E2E testing!

### Example implementations

- Noble Chain - <https://github.com/strangelove-ventures/noble>
- Juno Chain <https://github.com/CosmosContracts/juno>
- Go Relayer - <https://github.com/cosmos/relayer/blob/main/.github/workflows/interchaintest.yml>
- IBC-Go e2e tests - <https://github.com/cosmos/ibc-go/blob/main/.github/workflows/e2e-test-workflow-call.yml>
