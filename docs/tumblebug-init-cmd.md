# CB-Tumblebug Initialization Sub Command Guide

## Overview
Added a new `tumblebug-init` command under the `setup` subcommand to automatically initialize CB-Tumblebug with the currently running version. This feature ensures version compatibility and provides a streamlined initialization process.


## Prerequisites and Reference Information
This feature is an experimental add-on designed to make the `Initialize CB-Tumblebug to configure Multi-Cloud info` process more stable and convenient after all infrastructure has been built using the `./mayfly infra run` command.

The essential file required to properly execute the `tumblebug-init` command is an encrypted credential file, which is created through the following CB-Tumblebug process:
```
👉 Create your cloud credentials:
   ./init/genCredential.sh
   → Then edit ~/.cloud-barista/credentials.yaml with your CSP info.

👉 Encrypt the credentials file:
   ./init/encCredential.sh
```

If the above method does not create the file properly or if you need detailed information about the CB-Tumblebug initialization process, please refer to the [CB-Tumblebug Multi-Cloud Configuration Guide](https://github.com/cloud-barista/cb-tumblebug?tab=readme-ov-file#3-initialize-cb-tumblebug-to-configure-multi-cloud-info) documentation.


Additionally, since CB-Tumblebug uses uv internally, it is recommended to install uv in advance.
```
curl -LsSf https://astral.sh/uv/install.sh | sh
source $HOME/.local/bin/env
```
For more detailed information, please refer to the CB-Tumblebug documentation.

The `tumblebug-init` command must be executed with the `ubuntu` account added to the Docker group to run mayfly without sudo.
```
$ sudo usermod -aG docker $USER
$ newgrp docker
```


## Key Features

1. **Execution Status Check**: Verify if CB-Tumblebug is running
2. **Version Check**: Check the currently running CB-Tumblebug version
3. **Health Status Check**: Verify if CB-Tumblebug is in healthy state
4. **Guidance Message**: Provide related information and procedure guidance after version confirmation
5. **User Confirmation**: Confirm preparation of encrypted credential files
6. **Tumblebug Folder Check**: Check the exact Git tag (version) state of existing Tumblebug folder
7. **Smart Version Management**: Differentiated processing based on Git checkout state
8. **Intuitive Menu System**: Provide optimized selection options for each situation
9. **Download**: Download the version matching the currently running CB-Tumblebug from GitHub
10. **Execution Guidance**: Display information about the folder to be executed
11. **Initialization**: Run the cb-tumblebug init script — `multi-init.sh` (unified OpenBao + Tumblebug init, cb-tumblebug 0.12.9+) when present, or the legacy `init.sh` (0.12.8 and below) as a fallback. There is no `setup.env` sourcing; the script is run interactively so its prompts (encryption password, init.py fetch-method choice, confirmations) are answered directly by the user.
12. **Return**: Return to the original working directory

## Enhanced Capabilities

### 1. Health Status Validation
- Checks if CB-Tumblebug container is in healthy state before proceeding
- Prevents initialization attempts on unstable containers
- Provides clear guidance when container is not ready

### 2. OpenBao State Consistency (hard precondition)
- Before doing anything else, `tumblebug-init` runs an OpenBao state-consistency preflight and **aborts if OpenBao is not initialized or its `VAULT_TOKEN` is inconsistent** with the running container.
- Since cb-tumblebug 0.12.25 the credential registration performed during initialization writes to the OpenBao secret store; if the running cb-tumblebug container holds an empty or invalid token, that registration silently fails. The preflight catches this up front instead of letting the run flood with `VAULT_TOKEN is not set` errors.
- If you see `❌ OpenBao state is not consistent for tumblebug-init.`, follow the printed remediation. For a full-stack setup, `./mayfly infra run` performs the OpenBao initialization (Phase 0) before this command is needed; you can also run `./mayfly setup openbao init` / `./mayfly setup openbao status` directly.

### 3. Advanced Git Version Management
- **Precise Tag State Verification**: Verify that the current HEAD points exactly to a tag using `git describe --exact-match HEAD`
- **Git Checkout Functionality**: Switch to the desired version using `git checkout` command
- **Tag Existence Check**: Verify that the desired tag exists in the repository

### 4. Smart Directory Handling
- **Same Version (Exact Tag Checkout)**: No menu is shown. When the existing directory is already checked out at the exact tag of the running version, initialization proceeds automatically using that directory — there is nothing to decide. This is the common case, since `infra run` clones the matching tag on demand during OpenBao initialization.

- **Different Version or Tag Not Checked Out**:
  ```
  1. Delete and download fresh
  2. Switch to current version and continue initialization
  3. Switch to current version and exit
  0. Exit
  ```

### 5. User Experience Improvements
- **Clear Guidance Messages**: Provide specific explanations for each situation
- **Git Command Guide**: Present manual resolution methods
- **English Interface**: All messages unified in English
- **Intuitive Menu**: Improve user experience with number-based selection

## Technical Implementation

### New Functions Added
- `isTumblebugHealthy()`: Container health status validation
- `getExistingTumblebugVersion()`: Precise Git tag state checking
- `isTagExistsInRepo()`: Tag existence verification
- `showMenuAndHandleChoice()`: Context-aware menu system
- `switchToVersion()`: Git checkout functionality

### Error Handling
- Graceful handling of Git repository issues
- Clear error messages with resolution guidance
- Automatic directory restoration on errors

## Usage

```bash
# Basic usage
./mayfly setup tumblebug-init

# The command will:
# 1. Check if CB-Tumblebug is running and healthy
# 2. Verify OpenBao state is consistent (aborts otherwise)
# 3. Detect the current running version
# 4. Validate existing directory Git state
# 5. Provide appropriate options based on the situation
# 6. Execute initialization with proper version matching
```

## Benefits

1. **Version Safety**: Ensures exact version compatibility between running container and initialization script
2. **User Convenience**: Automated version detection and management
3. **Flexibility**: Multiple options for different scenarios
4. **Reliability**: Health checks and proper error handling
5. **Maintainability**: Clean, modular code structure

## Example Scenarios

### Scenario 1: Fresh Installation
```bash
✅ CB-Tumblebug is running.
✅ OpenBao VAULT_TOKEN is present and consistent.
✅ CB-Tumblebug is healthy.
✅ Version confirmed: 0.12.25
Downloading CB-Tumblebug v0.12.25 version from GitHub...
```

### Scenario 2: Existing Directory with Wrong Version
```bash
Different version of Tumblebug found in /path/to/cb-tumblebug folder.
Current running version: v0.12.25
Existing directory version: a1b2c3d4e5f6
The running version (v0.12.25) exists in the repository but is not currently checked out.

Please select an option:
1. Delete and download fresh
2. Switch to current version and continue initialization
3. Switch to current version and exit
0. Exit
```

## Troubleshooting
If CB-Tumblebug's API Password(`TB_API_PASSWORD`) is not set, the string value `"default"` is used as the password, so no error occurs. However, if CB-Tumblebug is running with a custom API Password, the API Password between the downloaded version and the running version may not match, which can cause errors.

Therefore, when executing the `./mayfly setup tumblebug-init` command, if an `Unauthorized` error occurs as shown below, please check the `TB_API_PASSWORD` configuration value:

```
 "Error during resource loading: 401 Client Error: Unauthorized for url: http://localhost:1323/tumblebug/loadAssets"
```

CB-Tumblebug API Password uses Hash verification method. For detailed information, please refer to the [TB API Password Configuration Guide](https://github.com/cloud-barista/cb-tumblebug/tree/main/cmd/bcrypt) documentation.


In cm-mayfly's `./cm-mayfly/conf/docker/docker-compose.yaml` file, the `cb-tumblebug` service is defined, and the `TB_API_PASSWORD` environment variable can be set in the `environment` section.
For the current CB-Tumblebug, if the `TB_API_PASSWORD` environment variable is not set, the string `default` is used as the `TB_API_PASSWORD` value.

Therefore, the quickest solution is to comment out the `TB_API_PASSWORD` environment variable in the `cb-tumblebug` service defined in the `./cm-mayfly/conf/docker/docker-compose.yaml` file if it is set, and then restart the service.

## Credential encryption password — resolution order

This is the password that decrypts your credential file during initialization (distinct from the `TB_API_PASSWORD` above). cm-mayfly does not handle or override it — it is resolved entirely inside CB-Tumblebug's `init/multi-init.sh` / `init.py`. As of the current CB-Tumblebug `init` scripts the order is, first match wins:

1. `--key-file <path>` argument passed to `init.py`
2. `~/.cloud-barista/.tmp_enc_key` key file (written by `encCredential.sh`)
3. `MULTI_INIT_PWD` environment variable
4. interactive prompt (`read -s`)

`tumblebug-init` runs the init script **interactively**: the script's prompts (the `MULTI_INIT_PWD` read, the `init.py` fetch-method choice, and any confirmations) are exposed to you and you answer them directly. cm-mayfly deliberately does **not** pre-feed fixed answers on stdin (e.g. `printf "y\n..." | ...`), because the prompt sequence is owned by CB-Tumblebug and changes by version/condition — a hardcoded answer would misalign and cause wrong answers, hangs, or "Invalid input" loops. `multi-init.sh` does begin with its own read for `MULTI_INIT_PWD`, but you simply answer it at the prompt; whatever is entered there does not decide the actual decryption — the key file (`.tmp_enc_key`) or `MULTI_INIT_PWD` is honoured downstream by `init.py`.

This order is owned by CB-Tumblebug and may change between versions — refer to the CB-Tumblebug `init` scripts for the authoritative behaviour. To avoid typing the password at the prompt, prepare the key file (via `encCredential.sh`) or export `MULTI_INIT_PWD` before running `tumblebug-init`; `init.py` then uses it for the decryption while you still answer the interactive prompts.