# Setup Windows Environment to Run Local Interchain

This is a step-by-step guide to setup a Windows environment and add the missing dependencies to run Local Interchain

**Local Interchain enables developers to :-**
- Quickly spin up a local testnet for any wasm chain.
- Test IBC connection compatibility on a high level between multiple local chains.
- Execute binary commands such as `tx decode` *[see example](../scripts/api_test.py)*.
- Store and execute wasm smart contracts *[see example](../scripts/daodao.py)*.
- Local RPC node + REST API which enables building scripts with any language.
- Configure chains to be launched within the testing environment, adjusting parameters such as gas options, number of validators, IBC paths, governance parameters, genesis accounts, and much more.
- Configure IBC relayers.

Local Interchain is the optimal playground for developers to flexibly test chain infrastructure and smart contract development.

This guide aims to configure a Windows OS environment to be compatible with Local Interchain.

This will allow wasm chains to run locally on a Windows system which has been unprecedented until now.

## Requirements

1. Make sure the Windows version is compatible with [Docker](https://www.docker.com/).

**Windows Desktop**: Win 7 or higher

**Windows Server**: Windows Server 2016 or higher

2. Enable Virtualization.
You can check if Virtualization is enabled by opening Task Manager and going to Performance as shown below.
If your Windows system is missing Hyper-V, you can follow [this guide](https://www.nakivo.com/blog/install-configure-hyper-v-manager/) to install and enable it.
![](https://i.imgur.com/8A6fRu0.png)

3. Permissions to run software installers and write to Program Files.

## Environment Setup

### 1. Installing Docker
Make sure you have wsl installed, you can check by running **Windows PowerShell** as adminstrator and running `wsl --install` which would install wsl2 if missing.

For Win 10 and Win 11, follow [this guide](https://docs.docker.com/desktop/install/windows-install/) to setup and install Docker Desktop on your Windows system.

For Windows Server, Win 7 and Win 8/8.1, use this [installation guide](https://github.com/microsoft/docker/blob/master/docs/installation/windows.md) for installing Docker Toolbox.

Confirm docker setup by running `docker version` in a cmd or a shell.

### 2. Installing Git
In case, you do not have Git already installed, use [this link](https://git-scm.com/download/win) to setup Git on Windows.

After installation, you will be able to run `git --version` in a cmd or a shell.

*Note: The guide assumes you installed Git under C:/Program Files/Git. If you have chosen a different installation location then use it instead.*

### 3. Installing Go
If you already have Go installed, run `go version` and make sure it is version1.19 or higher.

In case, you have an older version or do not have Golang installed, go to [golang downloads](https://go.dev/dl/) and download the featured Windows installer *.wsi*

After installation, open a new cmd or shell, and you will be able to run `go version`

### 4. Downloading Make
Make is a tool which controls the generation of executables and other non-source files of a program from the source files. It is necessary for building *`makefiles`*.

Make does not come with Windows, so we need to download the make binary which you can find provided by GNU [here](https://gnuwin32.sourceforge.net/packages/make.htm) and download the Binaries zip, or go to [this link](https://gnuwin32.sourceforge.net/downlinks/make-bin-zip.php) directly and begin downloading.

1. Extract the downloaded zip file
2. Go to the *`bin`*  folder, copy *`make.exe`*
3. Go to *`C:/Program Files/Git/mingw64/bin`* and paste *`make.exe`*
4. Through the control panel or your start menu, search and go to **Edit the system environment variables**, on Advanced tab, click Environment Variables, click Path and choose Edit, and add `C:\Program Files\Git\mingw64\bin`

![](https://i.imgur.com/dyK2YMm.png)

After finishing the steps above, open a new cmd or shell, and you will be able to run `make --version`

### 5. Installing GCC
GCC is the GNU compiler collection and it is necessary for compiling [cgo](https://pkg.go.dev/cmd/cgo).

GCC does not come with Windows, and the best compatible setup I've found was using [this guide](https://www.guru99.com/c-gcc-install.html) which works on both Windows Server and Windows Desktop including Win 11.

1. Go to [codeblocks](http://www.codeblocks.org/downloads/binaries/) and select the Windows option, then download the `mingw-setup.exe` binary.
2. Start installation.
3. Keep default component selection.

![](https://i.imgur.com/EeZUeJU.png)

4. Choose installation folder and click Install.
 
![](https://i.imgur.com/D25unIm.png)

5. After installation, no need to run *Code::Blocks* and click Next then Finish.

![](https://i.imgur.com/rvJmv9t.png)

6. Go to your installation folder i.e. *`C:\Program Files\CodeBlocks\MinGW\bin`* and you should find **gcc.exe**
7. Through the control panel or your start menu, search and go to **Edit the system environment variables**, on Advanced tab, click Environment Variables, click Path and choose Edit, and add `C:\Program Files\CodeBlocks\MinGW\bin` (choose the path from step 4 + \MinGW\bin)

After finishing the steps above, open a new cmd or shell, and you will be able to run `gcc --version`

## Running Local Interchain

1. Start the docker daemon by running **Docker Desktop** on Win 10/11 or using [**Docker Quickstart Terminal**](https://github.com/microsoft/docker/blob/master/docs/installation/windows.md#using-the-docker-quickstart-terminal) if you have installed Docker Toolbox.
2. Clone the Local Interchain Repo.
```bash
git clone https://github.com/strangelove-ventures/interchaintest.git 
cd interchaintest/local-interchain
```
3. Run `make install`
4. Run `local-ic start base.json`

Wait for it to set up and go to *https://127.0.0.1:8080/info*, you should see each local chain running in its own docker container `docker ps`

Now you are running a complete local wasm IBC-connected environment on a Windows operating system.

Happy building!
