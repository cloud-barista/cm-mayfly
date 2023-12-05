# cm-mayfly
The Operation Tool for Cloud-Migrator System Runtime

```
[NOTE]
cm-mayfly is currently under development.
So, we do not recommend using the current release in production.
Please note that the functionalities of cm-mayfly are not stable and secure yet.
If you have any difficulties in using cm-mayfly, please let us know.
(Open an issue or Join the Cloud-Migrator Slack)
```

## cm-mayfly 개요
- Cloud-Migrator 시스템의 설치, 실행, 상태정보 제공, 종료 등을 지원하는 관리 도구 입니다.
- 현재는 Docker Compose 모드 방식만 제공합니다.
  - [Docker Compose 모드](docs/cm-mayfly-docker-compose-mode.md)

## Install Docker & Docker Compose V2
- [Install Docker Engine on Ubuntu](https://docs.docker.com/engine/install/ubuntu/)
- Tested version: Docker version 24.0.7, build afdd53b
- Tested version: Docker Compose version v2.21.0

# Command to build the operator from souce code
```Shell
$ git clone https://github.com/cloud-barista/cm-mayfly.git

$ cd cm-mayfly/src

(Setup dependencies)
cm-mayfly/src$ go get -u

(Build a binary for cm-mayfly)
cm-mayfly/src$ go build -o mayfly
```

# Commands to use the mayfly

## Help
```
cm-mayfly/src$ ./mayfly 

The mayfly is a tool to operate Cloud-Barista system. 
  
  For example, you can setup and run, stop, and ... Cloud-Barista runtimes.
  
  - ./mayfly pull [-f ../docker-compose-mode-files/docker-compose.yaml]
  - ./mayfly run [-f ../docker-compose-mode-files/docker-compose.yaml]
  - ./mayfly info
  - ./mayfly stop [-f ../docker-compose-mode-files/docker-compose.yaml]
  - ./mayfly remove [-f ../docker-compose-mode-files/docker-compose.yaml] -v -i

Usage:
  mayfly [command]

Available Commands:
  help        Help about any command
  info        Get information of Cloud-Barista System
  pull        Pull images of Cloud-Barista System containers
  remove      Stop and Remove Cloud-Barista System
  run         Setup and Run Cloud-Barista System
  stop        Stop Cloud-Barista System

Flags:
      --config string   config file (default is $HOME/.mayfly.yaml)
  -h, --help            help for mayfly
  -t, --toggle          Help message for toggle

Use "mayfly [command] --help" for more information about a command.
```

## Run
```
cm-mayfly/src$ ./mayfly run -h

Setup and Run Cloud-Barista System

Usage:
  mayfly run [flags]

Flags:
  -f, --file string   Path to Cloud-Barista Docker-compose file (default "*.yaml")
  -h, --help          help for run

Global Flags:
      --config string   config file (default is $HOME/.mayfly.yaml)
```

## Stop
```
cm-mayfly/src$ ./mayfly stop -h

Stop Cloud-Barista System

Usage:
  mayfly stop [flags]

Flags:
  -f, --file string   Path to Cloud-Barista Docker-compose file (default "*.yaml")
  -h, --help          help for stop

Global Flags:
      --config string   config file (default is $HOME/.mayflyvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv.yaml)
```