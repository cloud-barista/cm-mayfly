# CM-Mayfly
The Operation Tool for Cloud-Migrator System Runtime

```
[NOTE]
CM-Mayfly is currently under development.
So, we do not recommend using the current release in production.
Please note that the functionalities of CM-Mayfly are not stable and secure yet.
If you have any difficulties in using CM-Mayfly, please let us know.
(Open an issue or Join the Cloud-Migrator Slack)
```

## CM-Mayfly Overview
This management tool provides and is expected to provide the following features:
- Builds and controls the infrastructure of the Cloud-Migrator system.
- Monitors the execution status of the sub-framework.
- Provides the ability to call REST APIs offered by the sub-framework.
- Kubernetes (k8s) will be supported in the future.


## CM-Mayfly Execution and Development Environment
- `Ubuntu 20.04` or later
  - Tested by Ubuntu 20.04
- `Golang 1.23` or later
  - Tested by go version go version go1.23.1 linux/amd64
- `Docker Compose v2.21` or later
  - Tested by Docker version 24.0.7, build afdd53b and Docker Compose version v2.21.0


## Pre-Install
- [Install Go](https://golang.org/doc/install) 
  - Optional and only necessary if you want to run or build the source code.
- [Install Docker Engine on Ubuntu](https://docs.docker.com/engine/install/ubuntu/)


## How to build mayfly binary file from souce code
Build a binary for mayfly using Makerfile
```Shell
$ git clone https://github.com/cloud-barista/cm-mayfly.git
$ cd cm-mayfly

Choose one of the commands below for the target OS you want to build for.
$ cm-mayfly$ make
$ cm-mayfly$ make win
$ cm-mayfly$ make mac
$ cm-mayfly$ make linux-arm
$ cm-mayfly$ make win86
$ cm-mayfly$ make mac-arm
```

## How to delete mayfly all binary files
```Shell
cm-mayfly$ make clean
```


# How to use CM-Mayfly
For now, it supports infra / rest / api / setup / tool sub-commands.   

Use the -h option at the end of the sub-command requiring assistance, or executing 'mayfly' without any options will display the help manual.   

```
$ ./mayfly -h
The mayfly is a tool to operate Cloud-Migrator system.

Usage:
  mayfly [command]

Available Commands:
  api         Call the Cloud-Migrator system's Open APIs as services and actions
  help        Help about any command
  infra       Installing and managing cloud-migrator's infrastructure
  rest        rest api call
  setup       Support for Additional Tasks After Container Setup
  tool        Provides additional functions for managing Docker Compose or the Cloud-Migrator system.

Flags:
  -h, --help   help for mayfly

Use "mayfly [command] --help" for more information about a command.
```

For more detailed explanations, see the articles below.   
- [infra sub-command guide](https://github.com/cloud-barista/cm-mayfly/blob/main/docs/cm-mayfly-infra.md)
- [setup sub-command guide](https://github.com/cloud-barista/cm-mayfly/blob/main/docs/cm-mayfly-setup.md)
- [rest sub-command guide](https://github.com/cloud-barista/cm-mayfly/blob/main/docs/cm-mayfly-rest.md)
- [api sub-command guide](https://github.com/cloud-barista/cm-mayfly/blob/main/docs/cm-mayfly-api.md)


# How to Build a Cloud-Migrator Infrastructure
`A quick guide` on how to easily build a Cloud-Migrator infrastructure.   
If you need a more detailed explanation, check out the article below.   
- [infra sub-command guide](https://github.com/cloud-barista/cm-mayfly/blob/main/docs/cm-mayfly-infra.md)


## Pre-Install
- [Install Docker Engine on Ubuntu](https://docs.docker.com/engine/install/ubuntu/)


## 1. Download cm-mayfly
```
$ git clone https://github.com/cloud-barista/cm-mayfly.git
$ cd cm-mayfly
```

## 2. Prerequisites
Some sub systems may require initial setup, including changing the default password. If changes or settings are needed, modify the information in the `./conf/docker` folder.

For example, to change the SMTP settings for cm-cicada, modify the following file:
`./conf/docker/conf/cm-cicada/airflow_smtp.env`

[For more details, refer to the cm-cicada SMTP configuration guide.](https://github.com/cloud-barista/cm-cicada?tab=readme-ov-file#smtp)

<!--
### mc-datamanager
The `mc-data-manager` subsystem `requires authentication information to use CSP`. Currently, only the configuration method using the `profile.json file` is supported. Therefore, if you wish to use mc-data-manager, `make sure to register the CSP-specific authentication information` in the `./conf/docker/conf/mc-data-manger/data/var/run/data-manager/profile/profile.json` file before setting up the infrastructure.   

If necessary, you can also modify the contents of the profile.json file after the infrastructure has been set up.
-->

## 3. Building a Docker-based infrastructure
In most cases, the following single line will complete all the necessary tasks.
```
$ ./mayfly infra run
```

If you do not want to see the output logs and want to run it in the background, you can use the `-d` option to run it in detach mode.
```
$ ./mayfly infra run -d
```


## 4. Checking the subsystem running status
To verify that the Cloud-Migrator system is running correctly, use the `info` command to check the healthy status of each subsystem.
```
$ ./mayfly infra info
```

## 5. Initialize CB-Tumblebug to configure Multi-Cloud info
To safely configure multi-cloud information, it is recommended to use the cb-tumblebug's official initialization guide instead of mayfly commands.

**Important**: It is crucial to use the exact version of cb-tumblebug that matches your running container to ensure compatibility and proper initialization.

First, check the version of the running cb-tumblebug container:
```
$ ./mayfly infra info -s cb-tumblebug
```

Example output:
```
[v]Status of Cloud-Migrator runtime images
CONTAINER           REPOSITORY                     TAG                 IMAGE ID            SIZE
cb-tumblebug        cloudbaristaorg/cb-tumblebug   0.11.9              d4c2abdc0e21        118MB
```

Based on the cb-tumblebug version (e.g., v0.11.9), download the corresponding cb-tumblebug repository:
```
$ git clone -b v0.11.9 https://github.com/cloud-barista/cb-tumblebug.git cb-tumblebug-v0.11.9
```

Then follow the detailed guide at:
[CB-Tumblebug Multi-Cloud Configuration Guide](https://github.com/cloud-barista/cb-tumblebug?tab=readme-ov-file#3-initialize-cb-tumblebug-to-configure-multi-cloud-info)


Alternatively, you can use the following experimental command to automatically download the source code matching the currently running cb-tumblebug version and execute the init.sh shell script.
```
$ ./mayfly setup tumblebug-init
```

For more detailed information, please refer to the [tumblebug-init Sub Command Guide](https://github.com/cloud-barista/cm-mayfly/blob/main/docs/tumblebug-init-cmd.md) documentation.

## 6. Some helpful commands
If a new version of the Docker image is released, you can update the running version of Cloud-Migrator to the latest version using the `update` command.
```
$ ./mayfly infra update
```

You can `update` a specific service using the `-s` flag.
```
$ ./mayfly infra update -s cb-spider
```
```
$ ./mayfly infra update -s "cb-spider cb-tumblebug"
```

You can check the logs of the entire system using the `logs` command.
```
$ ./mayfly infra logs
```

You can `logs` a specific service using the `-s` flag.
```
$ ./mayfly infra logs -s cb-spider
```
```
$ ./mayfly infra logs -s "cb-spider cb-tumblebug"
```



You can `stop` a specific service using the `-s` flag.
```
$ ./mayfly infra stop -s cb-spider
```
```
$ ./mayfly infra stop -s "cb-spider cb-tumblebug"
```

You can `run` a specific service using the `-s` flag.
```
$ ./mayfly infra run -s cb-spider
```
```
$ ./mayfly infra run -s "cb-spider cb-tumblebug"
```

## 7. Trouble Shooting
For some subsystems, including cm-cicada, the order of startup is important. Even if they are marked as healthy, they may not be running correctly. 
For cm-cicada, please check the logs and restart if any errors occur.
```
$ ./mayfly logs -s cm-cicada
```
Check if the number of Task Components in the Workflow Management menu on the web portal is 10 items.
Alternatively, you can easily check using the following curl command.
```
curl -s http://localhost:8083/cicada/task_component | jq '. | length'
```
If you determine that a restart is necessary, stop and then start it as shown below.
```
$ ./mayfly infra stop -s cm-cicada
$ ./mayfly infra run -s cm-cicada
```

If you want to cleanup all Docker environments, run the following shell script.

> [!CAUTION]
> **DANGER: This script will DELETE ALL Docker resources on your system!**
> 
> The `remove_all.sh` script is designed to create a clean environment for stable operation of Cloud-Migrator (cm-mayfly). It will remove **ALL Docker-related data** that was NOT installed through cm-mayfly, including:
> 
> - **ALL Docker containers** (running and stopped)
> - **ALL Docker images**
> - **ALL Docker volumes**
> - **ALL custom Docker networks**
> - **ALL Docker system data**
> 
> ⚠️ **This action is IRREVERSIBLE and will affect other Docker applications!**
> 
> Only use this script if you want to completely reset your Docker environment.

```
$ cd conf/docker
$ ./remove_all.sh
```