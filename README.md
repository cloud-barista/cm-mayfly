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
- This is a operate tool that supports installing, running, shutting down, providing status information, and open api service call the Cloud-Migrator system.
- As a proof-of-concept phase, only the `Docker Compose V2` mode method is currently available first.
- Support for k8s and the ability to make REST calls and Rest-based service calls will be developed in small increments.


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
For now, it supports docker / rest / api sub-commands.   

Use the -h option at the end of the sub-command requiring assistance, or executing 'mayfly' without any options will display the help manual.   

```
cm-mayfly/bin$ ./mayfly -h

The mayfly is a tool to operate Cloud-Migrator system.

Usage:
  mayfly [command]

Available Commands:
  api         Call the Cloud-Migrator system's Open APIs as services and actions
  docker      Installing and managing cloud-migrator's infrastructure
  help        Help about any command
  rest        rest api call

Flags:
  -h, --help   help for mayfly

Use "mayfly [command] --help" for more information about a command.
```

For more detailed explanations, see the articles below.   
- [docker sub-command guide](https://github.com/cloud-barista/cm-mayfly/blob/main/docs/cb-mayfly-docker-compose-mode.md)
- [rest sub-command guide](https://github.com/cloud-barista/cm-mayfly/blob/main/docs/cb-mayfly-rest.md)
- [api sub-command guide](https://github.com/cloud-barista/cm-mayfly/blob/main/docs/cb-mayfly-api.md)

## docker-compose.yaml
The necessary service information for the Cloud-Migrator System configuration is defined in the `cm-mayfly/conf/docker/docker-compose.yaml` file.(By default, it is set to build the desired configuration and data volume in the `conf/docker` folder.)   

If you want to change the information for each container you want to deploy, modify the `cm-mayfly/conf/docker/docker-compose.yaml` file or use the -f option.   



# docker subcommand
For now, it supports docker's run/stop/info/pull/remove commands.

Use the -h option at the end of the sub-command requiring assistance, or executing 'mayfly' without any options will display the help manual.   

```
Usage:
  mayfly docker [flags]
  mayfly docker [command]

Available Commands:
  info        Get information of Cloud-Migrator System
  pull        Pull images of Cloud-Migrator System containers
  remove      Stop and Remove Cloud-Migrator System
  run         Setup and Run Cloud-Migrator System
  stop        Stop Cloud-Migrator System

Flags:
  -h, --help   help for docker

Use "mayfly docker [command] --help" for more information about a command.
```
   
## docker subcommand examples
Simple usage examples for docker subcommand
```
 ./mayfly docker pull [-f ../conf/docker/docker-compose.yaml]   
 ./mayfly docker run [-f ../conf/docker/docker-compose.yaml]   
 ./mayfly docker info   
 ./mayfly docker stop [-f ../conf/docker/docker-compose.yaml]   
 ./mayfly docker remove [-f ../conf/docker/docker-compose.yaml] -v -i   
```


# k8s subcommand
K8S is not currently supported and will be supported in the near future.   



# rest subcommand
The rest subcommands are developed around the basic features of REST to make it easy to use the open APIs of Cloud-Migrator-related frameworks from the CLI.
For now, it supports get/post/delete/put/patch commands.

```
rest api call

Usage:
  mayfly rest [flags]
  mayfly rest [command]

Available Commands:
  delete      REST API calls with DELETE methods
  get         REST API calls with GET methods
  patch       REST API calls with PATCH methods
  post        REST API calls with POST methods
  put         REST API calls with PUT methods

Flags:
      --authScheme string   sets the auth scheme type in the HTTP request.(Exam. OAuth)(The default auth scheme is Bearer)
      --authToken string    sets the auth token of the 'Authorization' header for all HTTP requests.(The default auth scheme is 'Bearer')
  -d, --data string         Data to send to the server
  -f, --file string         Data to send to the server from file
  -I, --head                Show response headers only
  -H, --header strings      Pass custom header(s) to server
  -h, --help                help for rest
  -o, --output string       <file> Write to file instead of stdout
  -p, --password string     Password for basic authentication
  -u, --user string         Username for basic authentication
  -v, --verbose             Show more detail information

Use "mayfly rest [command] --help" for more information about a command.
```

## rest subcommand examples
Simple usage examples for rest subcommand
```
./mayfly rest get -u default -p default http://localhost:1323/tumblebug/health
./mayfly rest post https://reqres.in/api/users -d '{
                "name": "morpheus",
                "job": "leader"
        }'
```


# api subcommand
The api subcommands are developed to make it easy to use the open APIs of Cloud-Migrator-related frameworks from the CLI.

```
Call the action of the service defined in api.yaml. For example:
./mayfly api --help
./mayfly api --list
./mayfly api --service spider --list
./mayfly api --service spider --action ListCloudOS
./mayfly api --service spider --action GetCloudDriver --pathParam driver_name:AWS
./mayfly api --service spider --action GetRegionZone --pathParam region_name:ap-northeast-3 --queryString ConnectionName:aws-config01

Usage:
  mayfly api [flags]
  mayfly api [command]

Available Commands:
  tool        Swagger JSON parsing tool to assist in writing api.yaml files

Flags:
  -a, --action string        Action to perform
  -c, --config string        config file (default "../conf/api.yaml")
  -d, --data string          Data to send to the server
  -f, --file string          Data to send to the server from file
  -h, --help                 help for api
  -l, --list                 Show Service or Action list
  -m, --method string        HTTP Method
  -o, --output string        <file> Write to file instead of stdout
  -p, --pathParam string     Variable path info set "key1:value1 key2:value2" for URIs
  -q, --queryString string   Use if you have a query string to add to URIs
  -s, --service string       Service to perform
  -v, --verbose              Show more detail information

Use "mayfly api [command] --help" for more information about a command.
```

## api subcommand examples
Simple usage examples for api subcommand
```
./mayfly api --help
./mayfly api --list
./mayfly api --service spider --list
./mayfly api --service spider --action ListCloudOS
./mayfly api --service spider --action GetCloudDriver --pathParam driver_name:AWS
./mayfly api --service spider --action GetRegionZone --pathParam region_name:ap-northeast-3 --queryString ConnectionName:aws-config01
```
