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
For now, it supports docker / rest / api sub-commands.   

Use the -h option at the end of the sub-command requiring assistance, or executing 'mayfly' without any options will display the help manual.   

```
$ ./mayfly -h

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


# How to Build a Cloud-Migrator Infrastructure
A quick guide on how to easily build a Cloud-Migrator infrastructure.   
If you need a more detailed explanation, check out the article below.   
- [docker sub-command guide](https://github.com/cloud-barista/cm-mayfly/blob/main/docs/cb-mayfly-docker-compose-mode.md)


## 1. Download cm-mayfly
```
$ git clone https://github.com/cloud-barista/cm-mayfly.git
$ cd cm-mayfly
```

## 2. Prerequisites
Some subsystems require preliminary setup.
### mc-datamanager
The `mc-data-manager` subsystem `requires authentication information` to use CSP.
If you want to use mc-data-manager, make sure to refer to the contents of the `./conf/docker/conf/mc-data-manger/data/var/run/data-manager/profile/profile.json` file and `register the authentication information for each CSP`.


## 3. Building a Docker-based infrastructure
```
$ ./cm-mayfly docker run
```

## 4. Checking the subsystem running status
```
$ ./cm-mayfly docker info
```


<!-- 
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
 ./mayfly docker pull [-f ./conf/docker/docker-compose.yaml]   
 ./mayfly docker run [-f ./conf/docker/docker-compose.yaml]   
 ./mayfly docker info   
 ./mayfly docker stop [-f ./conf/docker/docker-compose.yaml]   
 ./mayfly docker remove [-f ./conf/docker/docker-compose.yaml] -v -i   
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
  -c, --config string        config file (default "./conf/api.yaml")
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

For more information, see the [API Sub Command Guide](https://github.com/cloud-barista/cm-mayfly/blob/main/docs/cb-mayfly-api.md).



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

Examples of changing REST authentication values   
Example of changing the username and password for basic authentication.   
`./mayfly api -s cm-ant -a getcostinfo --authUser=test --authPassword=test2`

Example of changing the authentication token for bearer authentication.   
`./mayfly api -s cm-ant -a getcostinfo --authToken=token`
-->