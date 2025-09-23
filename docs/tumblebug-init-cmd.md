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
11. **Initialization**: Execute init.sh after sourcing setup.env (with user input support)
12. **Return**: Return to the original working directory

## Enhanced Capabilities

### 1. Health Status Validation
- Checks if CB-Tumblebug container is in healthy state before proceeding
- Prevents initialization attempts on unstable containers
- Provides clear guidance when container is not ready

### 2. Advanced Git Version Management
- **Precise Tag State Verification**: Verify that the current HEAD points exactly to a tag using `git describe --exact-match HEAD`
- **Git Checkout Functionality**: Switch to the desired version using `git checkout` command
- **Tag Existence Check**: Verify that the desired tag exists in the repository

### 3. Smart Directory Handling
- **Same Version (Exact Tag Checkout)**:
  ```
  1. Delete and download fresh
  2. Use existing files
  0. Exit
  ```

- **Different Version or Tag Not Checked Out**:
  ```
  1. Delete and download fresh
  2. Switch to current version and continue initialization
  3. Switch to current version and exit
  0. Exit
  ```

### 4. User Experience Improvements
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
# 2. Detect the current running version
# 3. Validate existing directory Git state
# 4. Provide appropriate options based on the situation
# 5. Execute initialization with proper version matching
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
✅ CB-Tumblebug is healthy.
✅ Version confirmed: 0.11.9
Downloading CB-Tumblebug v0.11.9 version from GitHub...
```

### Scenario 2: Existing Directory with Wrong Version
```bash
Different version of Tumblebug found in /path/to/cb-tumblebug folder.
Current running version: v0.11.9
Existing directory version: a1b2c3d4e5f6
The running version (v0.11.9) exists in the repository but is not currently checked out.

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