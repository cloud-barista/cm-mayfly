## API 서브 명령 사용 가이드
이 가이드에서는 `cm-mayfly`의 `api` 서브 커맨드를 이용하여 `Name` 기반으로 Cloud-Migrator 시스템의 RESTful API를 실행하는 방법에 대해 소개합니다.    

`api` 서브 커맨드는 `rest` 서브 커맨드처럼 복잡한 Bear 인증 설정 및 URI를 외울 필요 없이 호출하려는 서비스의 이름(프레임워크 이름)과 액션 이름(호출할 API 이름)의 조합으로 Cloud-Migrator 시스템에서 제공하는 간단한 REST API들을 호출할 수 있도록 가볍게 제공되는 유틸성 기능입니다.

## 순서
1. 실행 환경 구축
1. 환경 파일 수정 및 경로
1. 환경 파일 구조
1. 사용 방법


## 실행 환경 구축
`cm-mayfly`의 `api` 서브 커맨드를 사용하기 위해서는 실행 파일을 다운로드하거나 소스를 빌드하는 방법이 있습니다.

`./bin` 폴더에 실행 파일이 있으며, `./conf` 폴더에 `api.yaml` 환경 설정 파일이 존재하므로 간단하게는 git에서 소스를 내려 받은 후 `./bin` 폴더에서 실행하면 됩니다.

```bash
$ git clone https://github.com/cm-mayfly/cm-mayfly.git
$ cd cm-mayfly/bin
$ ./mayfly api
```


만약, 소스를 수정하였거나 최신 소스를 직접 빌드하고 싶은 경우에는 README 설명을 참고하여 빌드하세요.   
[How to build mayfly binary file from souce code](https://github.com/MZC-CSC/cm-mayfly/tree/develop?tab=readme-ov-file#how-to-build-mayfly-binary-file-from-souce-code)



## 환경 파일 수정 및 경로
`api` 서브 커맨드는 호출할 서버의 정보를 비롯하여 서비스 명칭과 API 명칭 파악을 위해 내부적으로 `./conf/api.yaml` 환경 파일을 이용하기에 `api.yaml` 파일의 환경 정보가 사용하려는 시스템의 인프라 정보와 일치하도록 ``각 프레임워크의 실제 서버 IP및 Port 등의 정보에 맞게 수정``하시기 바랍니다.

만약, 다른 경로나 다른 환경 파일을 이용하고 싶은 경우 매번 --config 플래그 옵션을 사용하여 다른 경로의 설정 파일을 지정할 수 있습니다.


## 환경 파일 구조
api.yaml의 경우 아래와 같은 구조로 되어있습니다.   
`api.yaml`파일에 RESTful API를 제공하는 각 프레임워크의 서비스 명(services:)과 해당 서비스 하위에 존재할 서비스 액션(serviceActions:)을 지정하는 형태로 되어있습니다.   

### 서비스 정의
`services:` 하위에 `api` 서브 커맨드에서 지원할 프레임워크들의 정보를 서술합니다.   
예를 들어, 대표 프레임워크인 cb-spider의 경우 `cb-spider:`로 서술합니다.   

`baseurl`에는 해당 프레임워크에서 제공하는 REST API URI의 기본이되는 `스키마 + 호스트 + 베이스 경로`의 조합으로 기입합니다.   
예를 들어, cb-spider의 경우 localhost에서 1024포트를 사용하고 /spider 하위에 api가 존재한다면 `http://localhost:1024/spider`로 설정하면 됩니다.
   
   
**인증 정보 설정**   
현재 REST 호출을 위한 인증 절차는 "basic"과 "bearer" 인증을 지원하고 있습니다.   
해당 프레임워크에서 REST 호출을 위한 username과 password 기반의 기본 인증 절차가 필요한 경우에는 `auth` 영역의 `type`에는 `basic`을 입력하고 `username`과 `password` 항목으로 인증 정보를 지정하면됩니다.   
만약, `username`과 `password` 값을 api.yaml 파일의 값이 아닌 API 호출 시점에 설정하고 싶다면 `--authUser`와 `--authPassword`를 이용해서 변경 가능합니다.   
```
(예시)
./mayfly api -s 서비스명 -a 액션명 --authUser=User이름 --authPassword=비밀번호
```

해당 프레임워크에서 REST 호출을 위한 `token` 기반의 `bearer` 인증 절차가 필요한 경우에는 `auth` 영역의 `type`에는 `bearer`을 입력하고 `token`에 인증 토큰 정보를 지정하면됩니다.   
만약, `token` 값을 api.yaml 파일의 값이 아닌 API 호출 시점에 설정하고 싶다면 `--authUser`와 `--authPassword`를 이용해서 변경 가능합니다.   
```
(예시)
./mayfly api -s 서비스명 -a 액션명 --authToken=인증토큰값
```

별도의 인증 절차가 필요 없는 경우 `auth:` 항목만 유지하면됩니다.   
 `인증 절차가 필요 없는 경우에는 auth: 항목은 유지하고 하위의 내용만 삭제`하면됩니다.

**[인증 정보 설정 예시]**
```
  #none authentication method
    auth: #none

  #basic authentication method
    auth: 
      type: "basic"
      username: "your-username"
      password: "your-password"

  #Bearer authentication method
    auth: 
      type: "bearer"
      token: "your-bearer-token-here"
```


**[서비스 설정 예시]**
```
services:
  cb-spider: #service name
    baseurl: http://localhost:1024/spider  # baseurl is Scheme + Host + Base Path
    auth: #If you need an authentication method, describe the type and username and password in the sub
      type: basic
      username: default
      password: default
  
```

### 서비스의 액션 정의 
`serviceActions:` 영역은 `services:` 에서 정의된 특정 서비스에서 제공하는 REST API들에 대해서 액션으로 정의하는 영역입니다.   

예를 들어, 위에서 설명했던 `cb-spider:` 서비스(프레임워크)에서 제공하는 
API들을 정의하고 싶다면 아래처럼 정의합니다.
```
serviceActions:
  cb-spider: 
```

이 영역 하위에 해당 서비스에서 제공하는 모든 API들을 아래와 같은 형태로 하나씩 서술하면됩니다.   
해당 서비스 하위에 1:1 맵핑할 API의 액션 이름을 서술하고 하위에 `method` 영역에는 호출될 REST Method 방식을 지정하고 `resourcePath` 영역에는 최종 URL을 만들기 위해 서비스 영역에서 정의한 **baseurl** 이후의 남은 endpoint까지의 전체 URI를 서술합니다.   
끝으로, 필요한 경우에는 description에 간단한 설명을 기술합니다.(현재는 미사용)   

**[액션 정의 형태]**

```
영문 액션명:
    method: REST메소드 방식(get/put/post 등)
    resourcePath: baseurl 이후의 URI 경로
    description: 액션에 대한 설명
```

예를 들어, cb-spider에서 제공하는 `http://localhost:1024/spider/cloudod`라는 REST API를 `ListCloudOS`라는 액션 이름으로 지정하고 싶다면, cb-spider의 경우 `services:` 영역에 `cb-spider:`로 정의되어 있으며 `http://localhost:1024/spider` 까지는 `baseurl`에 지정되어 있기 때문에 남은 URI의 경우 `/cloudos`이며 GET 방식으로 호출하기 때문에 최종적으로는 아래처럼 정의합니다.   

```
serviceActions:
  cb-spider:
    ListCloudOS:
      method: get
      resourcePath: /cloudos
```

동일한 형식으로, 이번에는 `http://localhost:1024/spider/driver/aws` 또는 `http://localhost:1024/spider/driver/gcp`처럼 cb-spider에서 제공하는 REST API중 URI가 가변되는 Get 방식의 REST API를 `GetCloudDriver`라는 액션 이름으로 지정하고 싶다면 아래처럼 가변 부분을 {}로 정의합니다.   

```
serviceActions:
  cb-spider:
    GetCloudDriver:
      method: get
      resourcePath: /driver/{driver_name}
```

`{driver_name}`처럼 {}로 지정된 액션의 경우에는, {}안에 있는 변수명이 매개변수 이름이되며, 사용자로부터 `-p` 또는 `--pathParam` 플래그를 이용해서 `--pathParam driver_name:AWS` 형태로 값을 전달 받을 수 있습니다.

```
$ ./mayfly api --service spider --action GetCloudDriver --pathParam driver_name:AWS

$ ./mayfly api -s spider -a GetCloudDriver -p driver_name:AWS

$ ./mayfly api -s spider -a GetCloudDriver -p driver_name:GCP
```

예로 설명드렸던, ListCloudOS와 GetCloudDriver 액션들의 최종 정의 형태는 아래와 같습니다.
```
serviceActions:
  cb-spider:
    ListCloudOS:
      method: get
      resourcePath: /cloudos
    GetCloudDriver:
      method: get
      resourcePath: /driver/{driver_name}
```

Cloud-Migrator 시스템의 경우 대부분의 프레임워크는 `Swagger` 기반으로 진행되고 있어서 Swagger 형태의 JSON 파일을 제공하고 있으므로 `api.yaml`파일의 설정 작업을 조금은 수월하게 진행할 수 있도록 `tool` 플래그를 제공하고 있으니 유사한 환경의 다른 프레임워크의 정보를 api.yaml에 정의하고 싶다면 tool 플래그를 활용하시기 바랍니다.
```
$ ./mayfly api tool -f 스웨거파일.json
```

참고로, `tool` 플래그의 경우, 내부적으로 각 API의 액션 이름에 `operationId` 필드를 이용하기 때문에 `사용하려는 Swagger JSON 파일에는 반드시 operationId 필드가 정의`되어 있어야 합니다.   
**[예시]**
```
"basePath": "/tumblebug",
"paths": {
	"/availableK8sClusterNodeImage": {
		"get": {
			"description": "Get available kubernetes cluster node image",
			"summary": "Get available kubernetes cluster node image",
			"operationId": "GetAvailableK8sClusterNodeImage",
```



Swagger 파일을 제공하는 cb-tumblebug의 경우 tool 플래그를 실행하면 아래와같은 형태로 출력됩니다.
```
$ ./mayfly api tool -f ./cb-tumblebug.json
```

**[실행 결과 예시]**
```
Base Paht: /tumblebug
    GetAvailableK8sClusterNodeImage:
      method: get
      resourcePath: /availableK8sClusterNodeImage
      description: "Get available kubernetes cluster node image"
                . . . .
                 생략
                . . . .
    GetCloudInfo:
      method: get
      resourcePath: /cloudInfo
      description: "Get cloud information"
    GetAllConfig:
      method: get
      resourcePath: /config
      description: "List all configs"
                . . . .
                 생략
                . . . .
```
위 출력 결과를 참고해서 api.yaml 파일을 작성하면됩니다.


## 사용 방법
-s 또는 --service 플래그를 이용해서 호출을 희망하는 프레임워크를 설정하며, -a 또는 --action 플래그를 이용해서 해당 프레임워크에서 실제 호출하려는 API를 지정합니다.   

만약, 호출하려는 액션의 URI 경로가 가변 경로인 경우 --pathParam으로 경로 설정이 가능하며, 전달할 JSON Data 가 있는 경우 --data 또는 --file 플래그를 이용할 수 있습니다.   

그 외 일부 편의를 위한 플래그가 제공됩니다.
```
Call the action of the service defined in api.yaml. For example:
$ ./mayfly api --help
$ ./mayfly api --list
$ ./mayfly api --service spider --list
$ ./mayfly api --service spider --action ListCloudOS
$ ./mayfly api --service spider --action GetCloudDriver --pathParam driver_name:AWS
$ ./mayfly api --service spider --action GetRegionZone --pathParam region_name:ap-northeast-3 --queryString ConnectionName:aws-config01

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

### 주요 flag 설명
--service (-s) : 호출할 서비스의 이름을 지정합니다.   
--action (-a) : 호출할 액션의 이름을 지정합니다.   
--pathParam (-p) : 경로 파라미터를 지정합니다.   
--queryString (-q) : 쿼리 문자열을 지정합니다.   
--data (-d) : 서버에 전송할 데이터를 지정합니다.   
--file (-f) : 서버에 전송할 데이터가 포함된 파일을 지정합니다.   
--output (-o) : 서버 응답을 파일에 저장합니다.   
--config (-c) : 사용할 환경 설정 파일을 지정합니다. (기본값 "./conf/api.yaml")
--list (-l) : 사용 가능한 서비스 목록 또는 액션 목록을 조회합니다.   


### 기본 사용 형식
액션만 필요한 경우 서비스와 액션만 지정하면됩니다.
```
$ ./mayfly api --service <service_name> --action <action_name>

$ ./mayfly api -s <service_name> -a <action_name>
```

만약, 가변 경로와 쿼리 스트링이 존재하는 경우 아래처럼 지정합니다.   
전달할 경로 파라메터가 많은 경우 "key1:value1 key2:value2"처럼 각각은 공백으로 구분합니다.
```
$ ./mayfly api --service <service_name> --action <action_name> --pathParam <key1:value1 key2:value2> --queryString <query_string>

$ ./mayfly api -s <service_name> -a <action_name> -p <key1:value1 key2:value2> -queryString <query_string>
```

### 환경 설정 파일과 실행 명령의 맵핑 관계
지금까지 설명드린 api.yaml에 정의되는 방식 및 CLI에서 호출하는 api 서브 커맨드의 맵핑 관계는 다음과 같습니다.
![apicall](https://github.com/cloud-barista/cm-mayfly/assets/78469943/17fe459b-cdab-402d-b87d-8f4ae12de646)


### 기본 사용 예시 
```
$ ./mayfly api --help
$ ./mayfly api --list
$ ./mayfly api --service cb-spider --list
$ ./mayfly api --service cb-spider --action ListCloudOS
$ ./mayfly api --service cb-spider --action GetCloudDriver --pathParam driver_name:AWS
$ ./mayfly api --service cb-spider --action GetRegionZone --pathParam region_name:ap-northeast-3 --queryString ConnectionName:aws-config01
```


### 실행 결과 예시
```
$ ./mayfly api --service cb-spider  --action ListCloudOS

service calling...
2024/05/02 09:39:33.922447 WARN RESTY Using Basic Auth in HTTP mode is not secure, use HTTPS
{"cloudos":["AWS","AZURE","GCP","ALIBABA","TENCENT","IBM","OPENSTACK","CLOUDIT","NCP","NCPVPC","NHNCLOUD","KTCLOUD","KTCLOUDVPC","DOCKER","MOCK","CLOUDTWIN"]}
```
