# Integrate E2E Tests Locally and in GitHub Action

### Goal:

Seamlessly build and test current iterations of your chain both locally and within GitHubs continuous integration (CI) pipeline.

Example workflow accomplished from this guide:


1. Chain code altered
2. Dev runs `make local-image` -> builds current iteration of chain inside docker image
3. Dev runs `make test-the-thing` -> runs test using new image
4. -- Development done, code pushed, PR created -- 
5. GitHub Action Builds Docker image (on PR)
6. GitHub Action runs tests as part of CI (on PR)


### Setup

We recommend creating a separate directory in your chain's repo and importing `interchaintest` as its own module. This will allow you to keep the extra imports needed for `interchaintest` separate from your chain.

See [`noble`](https://github.com/strangelove-ventures/noble) chains `interchaintest` [folder](https://github.com/strangelove-ventures/noble/tree/main/interchaintest) as an example.

The noble chains CI workflow is a great example to follow along with throughout this guide.


## Building Locally

**Goal:** Dev runs `make local-image` to create a docker container of the chains current code base. They can then use that image in the E2E tests.



`interchaintest` relies on docker images containing your chains binary. 

We recommend leveraging [`heighliner`](https://github.com/strangelove-ventures/heighliner) to build the docker image. Ideally you won't even need a local Dockerfile in your repo as Heighliner covers this.


Add this to  your Makerfile:

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
`make local-image` will build image: `noble:local`.

You'll need to change `-c` arg to your chain name, and the `--binaries` arg to your binary's install location.

It's important to realize the image will be named using the arg from the `-c` flag. The `--local` flag builds from your local repository (as opposed to the remote git repository) and tags the docker image as `local`. The image name and tag will be integrated into your tests [in the step below]()


Heighliner works out of the box with most Cosmos based chains. Other chains or non standard Cosmos chains may require extra args. See the Heighliner repo for more info.



## GitHub E2E Workflow for PR's

There are two jobs in this workflow. We recommend having this workflow run only on PR's.
1. Build Image based off pushed code and upload image as a GitHub Artifact 
2. Download artifact to runners and run tests

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
        uses: actions/upload-artifact@v3
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
        uses: actions/download-artifact@v3
        with:
          name: noble-docker-image

      - name: Load Docker Image
        run: docker image load -i ${{ env.TAR_PATH }}

      - name: run test
        run: make ${{ matrix.test }}
```
</details>

### Build Image

We recommend to use the [Heighliner Action](https://github.com/strangelove-ventures/heighliner-build-action) (strangelove-ventures/heighliner-build-action@v0.0.3). This action streamlines setting up Go, checking out the repo, and building the image with the proper tags.

Note the `tar-export-path`. This tarball archive of the docker image will be used by other GitHub runners.

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

As noted in the previous step, unless you are working with a non Cosmos chain or a non standard Cosmos chain, the only args you'll likely need to alter are the `chain` and `binaries` arguments.


### Upload Docker Image Tarball as GitHub artifact

This is the next step in the `build-docker` image job:

```yaml
...

env:
  TAR_PATH: heighliner.tar

jobs:
  build-docker:
...

      - name: Publish Tarball as Artifact
        uses: actions/upload-artifact@v3
        with:
          name: chain-docker-image
          path: ${{ env.TAR_PATH }}

```

You shouldn't need to change anything in this step.

### Run E2E Tests

The next job synchronously spins up a runner for each test. It then downloads the tarball docker image, loads the image into docker and runs all the specified tests.

The example below will spin up 9 runners.

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
        uses: actions/download-artifact@v3
        with:
          name: chain-docker-image

      - name: Load Docker Image
        run: docker image load -i ${{ env.TAR_PATH }}

      - name: run test
        run: make ${{ matrix.test }}
```

The only thing you'll need to change are the `test` names in the strategy matrix. These should match `Makerfile` commands. 

For example in the Makefile:

```makefile
ictest-tkn-factory:
	cd interchaintest && go test -race -v -run ^TestNobleChain$$ .
```

## Integrate Docker Name and Tag into Tests

The part that ties this all together is to specify the proper Docker `Repository` (image name) and `Version` (image tag) in your `ibc.ChainConfig`


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
> You may see a `ERROR	Failed to pull image` when first starting your test. This is expected when running local images.




### Example implementations:
- Noble Chain - https://github.com/strangelove-ventures/noble
- Juno Chain https://github.com/CosmosContracts/juno
- Go Relayer - https://github.com/cosmos/relayer/blob/main/.github/workflows/interchaintest.yml
- IBC-Go e2e tests - https://github.com/cosmos/ibc-go/blob/main/.github/workflows/e2e-test-workflow-call.yml 