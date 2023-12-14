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
- This is a operate tool that supports installing, running, providing status information, and shutting down the Cloud-Migrator system.
- As a proof-of-concept phase, only the `Docker Compose V2` mode method is currently available first.


## CM-Mayfly Execution and Development Environment
- `Ubuntu 20.04` or later
  - Tested by Ubuntu 20.04
- `Golang 1.19` or later
  - Tested by go version go1.19.2 linux/amd64
- `Docker Compose v2.21` or later
  - Tested by Docker version 24.0.7, build afdd53b and Docker Compose version v2.21.0


## Pre-Install
- [Install Go](https://golang.org/doc/install)
- [Install Docker Engine on Ubuntu](https://docs.docker.com/engine/install/ubuntu/)


## How to build mayfly binary file from souce code
```Shell
$ git clone https://github.com/cloud-barista/cm-mayfly.git

$ cd cm-mayfly/src

(Setup dependencies)
cm-mayfly/src$ go get -u

(Build a binary for mayfly)
cm-mayfly/src$ go build -o mayfly

(Build a binary for mayfly using Makerfile)
cm-mayfly/src$ make
cm-mayfly/src$ make win
cm-mayfly/src$ make mac
cm-mayfly/src$ make linux-arm
cm-mayfly/src$ make win86
cm-mayfly/src$ make mac-arm

(Delete all a binary for mayfly using Makerfile)
cm-mayfly/src$ make clean
```


# How to use CM-Mayfly
For now, it supports docker's run/stop/info/pull commands, and k8s is a work in progress.   

## docker-compose.yamlk
The necessary service information for the Cloud-Migrator System configuration is defined in the `cm-mayfly/docker-compose-mode-files/docker-compose.yaml` file.(By default, it is set to build the desired configuration and data volume in the `docker-compose-mode-files` folder.)   

If you want to change the information for each container you want to deploy, modify the `cm-mayfly/docker-compose-mode-files/docker-compose.yaml` file or use the -f option.   


## Help
Use the -h option at the end of the sub-command requiring assistance, or executing 'mayfly' without any options will display the help manual.   

```
cm-mayfly/src$ ./mayfly

The mayfly is a tool to operate Cloud-Migrator system.

  For example, you can setup and run, stop, and ... Cloud-Migrator runtimes.

  - ./mayfly pull [-f ../docker-compose-mode-files/docker-compose.yaml]
  - ./mayfly run [-f ../docker-compose-mode-files/docker-compose.yaml]
  - ./mayfly info
  - ./mayfly stop [-f ../docker-compose-mode-files/docker-compose.yaml]
  - ./mayfly remove [-f ../docker-compose-mode-files/docker-compose.yaml] -v -i

Usage:
  mayfly [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  info        Get information of Cloud-Migrator System
  pull        Pull images of Cloud-Migrator System containers
  remove      Stop and Remove Cloud-Migrator System
  run         Setup and Run Cloud-Migrator System
  stop        Stop Cloud-Migrator System

Flags:
  -h, --help   help for mayfly

Use "mayfly [command] --help" for more information about a command.
```

## Run
Create and start containers from an image of the Cloud-Migrator System packages   

```
cm-mayfly/src$ ./mayfly run -h

Setup and Run Cloud-Migrator System

Usage:
  mayfly run [flags]

Flags:
  -f, --file string   User-defined configuration file (default "Not_Defined")
  -h, --help          help for run
```

## Stop
Stop and remove containers, networks   

```
cm-mayfly/src$ ./mayfly stop -h

Stop Cloud-Migrator System

Usage:
  mayfly stop [flags]

Flags:
  -f, --file string   User-defined configuration file (default "Not_Defined")
  -h, --help          help for stop
```


## Info
Get information of Cloud-Migrator System Information about containers and container images

```
cm-mayfly/src$ ./mayfly info -h

Get information of Cloud-Migrator System. Information about containers and container images

Usage:
  mayfly info [flags]

Flags:
  -f, --file string   User-defined configuration file (default "Not_Defined")
  -h, --help          help for info
  ```


## Pull
```
cm-mayfly/src$ ./mayfly pull -h

Pull images of Cloud-Migrator System containers

Usage:
  mayfly pull [flags]

Flags:
  -f, --file string   User-defined configuration file (default "Not_Defined")
  -h, --help          help for pull
```
