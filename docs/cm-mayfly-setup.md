
## setup 명령 사용 가이드

`setup` 서브 커맨드는 Cloud-Migrator 시스템 설치(구축) 이후 설정을 보조하기 위해 가볍게 제공되는 기능으로서 이 가이드에서는 `cm-mayfly`의 `setup` 서브 커맨드를 이용하여 Cloud-Migrator 시스템에 특정 CSP의 Credential 정보 및 공통 리소스를 등록하는 방법에 대해 소개합니다.    


## 순서
1. setup 명령 기능
1. Credential 등록 방법
1. 공통 리소스 등록 방법


## setup 명령 기능
Cloud-Migrator 시스템을 이용하기 위해서는 사용하려는 CSP의 Credential 정보와 공통 리소스 등록이 필요합니다.

이를 위해 cb-tumblebug에서는 [init script](https://github.com/cloud-barista/cb-tumblebug/tree/main/init)를 제공하며 setup 커맨드의 credential 커맨드가 init script에서 진행하는 Credential 등록 기능과 동일합니다.


```
$ ./mayfly setup
Supports installation tasks for specific containers after setting up the Cloud-migrator's infrastructure.

Usage:
  mayfly setup [flags]
  mayfly setup [command]

Available Commands:
  credential  Registration of CSP-Specific Credentials and Default Resources

Flags:
  -h, --help   help for setup

Use "mayfly setup [command] --help" for more information about a command.
```


## Credential 등록 방법

cb-tumblebug에서 제공하는 `공개키 기반의 암호화 방식`으로 `CSP의 Credential 정보를 등록`합니다.
자세한 사용 방법은 `-h` 옵션을 입력하면 `세부 flag`를 살펴 볼 수 있으며, 보통은 `./conf/api.yaml` 설정 파일을 이용해서 기본 정보를 사용하기 때문에 `단순히 credential 커맨드만 입력`하면 필요한 절차를 실행합니다.

현재 지원되는 세부 기능은 아래와 같습니다.
```
$ ./mayfly setup credential -h
Supports the registration of CSP credentials and initial data
        The basic information of the subsystem is utilized from the api.yaml file, but the user can change the API authentication information including host and port.

Usage:
  mayfly setup credential [flags]

Flags:
      --authToken string   sets the auth token of the 'Authorization' header for all HTTP requests.(The default auth scheme is 'Bearer')
  -c, --config string      config file (default "./conf/api.yaml")
      --csp string         The cloud service provider (CSP) to register
  -H, --header strings     Pass custom header(s) to server
  -h, --help               help for credential
      --host string        The server address where Tumblebug is running (Default: localhost) (default "localhost")
  -p, --password string    Password for basic authentication
      --port string        The port number Tumblebug is using (Default: 1323) (default "1323")
  -u, --user string        Username for basic authentication
  -v, --verbose            Show more detail information
```

아무런 옵션 없이 credential 명령만 실행하면 등록 가능한 CSP 목록을 조회해서 선택할 수 있도록 Guide 하며, 선택된 CSP에 맞는 입력 값을 요청합니다.

```
$ ./mayfly setup credential
```

또는 --csp 플래그를 이용해서 특정 CSP를 직접 지정할 수 있습니다.
```
./mayfly setup credential --csp aws
```

*[주의] 비밀키 등 민감한 정보의 경우 입력된 Key값이 노출되지 않도록 콘솔에 입력된 정보가 출력되지 않도록 했기 때문에 `절대로 입력 도중 Ctrl+C 키를 누르면 안됩니다.`*

**실행 예시**
```
./mayfly setup credential
Configuration file[./conf/api.yaml] processing...
Configuration file[./conf/api.yaml] processed.
Tumblebug Base URL : http://localhost:1323/tumblebug

Available CSPs:
1. alibaba
2. aws
3. azure
4. gcp
5. ibm
6. ktcloud
7. ktcloudvpc
8. ncp
9. ncpvpc
10. nhncloud
11. openstack
12. tencent
0. Exit
Please select a CSP by number: 2

Processing authentication information for selected [aws] CSP
Retrieving credential input format for aws
Successfully retrieved credential meta information for aws
Please enter ClientId:
Please enter ClientSecret:
Do you want to review the entered credentials? (yes/no): no
Is this correct? (yes/no/retry): yes
```



## 공통 리소스 등록 방법 
사용하려는 CSP들의 Credential 정보가 등록되었으면 `cb-tumblebug에서 제공하는 loadassets Rest API를 호출`하면 cb-tumblebug 자체에 존재하는 로컬 파일 기반으로 필요한 공통 리소스 정보가 등록됩니다.
cm-mayfly 에서는 `api 서브 커맨드`를 제공하므로 api 커맨드를 이용해서 손쉽게 호출 가능합니다.

```
$ ./mayfly api -s cb-tumblebug -a loadassets
```

`api 서브 커맨드`에 대한 보다 자세한 내용은 [api sub-command guide](https://github.com/cloud-barista/cm-mayfly/blob/main/docs/cm-mayfly-api.md) 문서를 살펴 보시기 바랍니다.


