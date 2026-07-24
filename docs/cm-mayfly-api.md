## API 서브 명령 사용 가이드
이 가이드에서는 `cm-mayfly`의 `api` 서브 커맨드를 이용하여 Cloud-Migrator 시스템의 RESTful API를 `API Name` 기반으로 호출하는 방법에 대해 소개합니다.    

`api` 서브 커맨드는 Cloud-Migrator의 서브 시스템들이 제공하는 REST API를 `rest` 서브 커맨드처럼 복잡한 Bear 인증 설정 및 URI를 직접 외울 필요 없이 `호출하려는 서비스의 이름`(서브 시스템 이름)과 `액션 이름`(호출할 API 이름)으로 REST API를 간단하게 호출할 수 있도록 가볍게 제공되는 유틸성 기능입니다.   
참고로, 서브 시스템은 cb-spider나 cb-tumblebug처럼 Cloud-Migrator 시스템을 구성하고 있는 프레임워크들입니다.

## 순서
1. 실행 환경 구축
1. 환경 파일 수정 및 경로
1. 환경설정(api.yaml) 파일 구조
1. 사용 방법


## 실행 환경 구축
`api` 서브 커맨드를 사용하기 위해서는 실행 파일을 다운로드하거나 소스를 빌드하는 방법이 있습니다.
간단하게 git으로 소스를 내려 받은 후 실행하면 됩니다.

```bash
$ git clone https://github.com/cloud-barista/cm-mayfly.git
$ cd cm-mayfly
$ ./mayfly api
```

만약, 소스를 수정하였거나 최신 소스를 직접 빌드하고 싶은 경우에는 README 설명을 참고하여 빌드하세요.   
[How to build mayfly binary file from souce code](https://github.com/MZC-CSC/cm-mayfly/tree/develop?tab=readme-ov-file#how-to-build-mayfly-binary-file-from-souce-code)



## 환경 파일 수정 및 경로
`api` 서브 커맨드는 호출할 서브 시스템의 정보를 비롯하여 서비스 명칭과 API 명칭 파악을 위해 `./conf/api.yaml` 환경 파일을 이용합니다.
따라서, 반드시 `api.yaml` 파일의 환경 정보가 사용하려는 시스템의 인프라 정보와 일치하도록 ``각 서브 시스템의 실제 서버 IP및 Port 등의 정보에 맞게 수정``하시기 바랍니다.

만약, 기본 파일이 아닌 사용자가 원하는 환경 설정 파일을 이용하고 싶은 경우에는 `mayfly`를 실행할 때 --config 플래그 옵션을 이용하여 원하는 환경 설정 파일을 지정할 수 있습니다.


## 환경설정(api.yaml) 파일 구조
api.yaml 파일의 경우 아래와 같은 구조로 되어있습니다.   
RESTful API를 제공하는 각 서브 시스템의 `서비스 명(services:)`과 서브 시스템에서 제공하는 API에 대한 `서비스 액션(serviceActions:)`을 지정하는 형태로 되어있습니다.   


### 서비스 정의
`services:` 하위에 api 서브 커맨드에서 지원할 `서브 시스템들의 정보`를 서술합니다.   
예를 들어, 대표 프레임워크인 cb-spider의 경우 `cb-spider:`로 서술합니다.   

`baseurl`에는 해당 프레임워크에서 제공하는 REST API URI의 기본이되는 `스키마 + 호스트 + 베이스 경로`의 조합으로 기입합니다.   
예를 들어, cb-spider의 경우 localhost에서 1024포트를 사용하고 /spider 하위에 api가 존재한다면 `http://localhost:1024/spider`로 설정하면 됩니다.
   
   
**인증 정보 설정**   
현재 mayfly에서는 REST 호출을 위한 "basic"과 "bearer" 인증을 지원하고 있으며, 서브 시스템에 따라서는 REST API를 호출할 때 인증 정보가 필요한 경우도 있고 인증 정보가 필요 없는 경우도 있습니다.   

[basic인증]   
REST 호출을 위한 username과 password 기반의 기본 인증 절차가 필요한 경우에는 `auth` 영역의 `type`에는 `basic`을 입력하고 `username`과 `password` 항목으로 인증 정보를 지정하면됩니다.   
만약, `username`과 `password` 값을 api.yaml 파일의 값이 아닌 API 호출 시점에 설정하고 싶다면 `--authUser`와 `--authPassword`를 이용해서 변경 가능합니다.   
```
(호출 시점 인증 정보 지정 예시)
./mayfly api -s 서비스명 -a 액션명 --authUser=User이름 --authPassword=비밀번호
```

[bearer인증]   
서브 시스템에서 REST 호출을 위한 `token` 기반의 `bearer` 인증 절차가 필요한 경우에는 `auth` 영역의 `type`에는 `bearer`을 입력하고 `token`에 인증 토큰 정보를 지정하면됩니다.   
만약, `token` 값을 api.yaml 파일의 값이 아닌 API 호출 시점에 설정하고 싶다면 `--authToken`을 이용해서 변경 가능합니다.   
```
(호출 시점 인증 정보 지정 예시)
./mayfly api -s 서비스명 -a 액션명 --authToken=인증토큰값
```

[REST 인증이 필요 없는 경우]   
REST 호출에 별도의 인증 절차가 필요 없는 경우에는 `auth: 항목은 유지하고 하위의 내용만 삭제`하면됩니다.

**[다양한 인증 정보 설정 예시]**
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

**[환경 변수 참조(`${VAR}`) 방식]**   
`username`·`password`·`token` 같은 자격증명 값은 위 예시처럼 평문으로 직접 적을 수도 있지만, `${VAR}` 형태로 **환경 변수를 참조**하도록 적을 수도 있습니다. 실제로 배포되는 `conf/api.yaml`은 자격증명을 파일 밖으로 빼기 위해 이 방식을 사용합니다. 예:
```
services:
  cb-spider:
    baseurl: http://cb-spider:1024/spider
    auth:
      type: basic
      username: ${SPIDER_USERNAME}
      password: ${SPIDER_PASSWORD}
```
값이 `${VAR}`로 적혀 있으면 호출 시점에 다음 순서로 해석되며, 먼저 찾은 값이 사용됩니다.
1. CLI 플래그(`--authUser`/`--authPassword`/`--authToken`) — 지정 시 항상 우선
2. 프로세스(OS) 환경 변수
3. `conf/docker/.env` 파일
4. 어디에도 없으면 빈 값으로 처리되고 경고가 출력됩니다(암묵적 기본값은 없습니다). basic 인증에서는 Authorization 헤더가 전송되지 않습니다.

### 서비스의 액션 정의 
`serviceActions:` 영역은 `services:` 에서 정의된 특정 서비스에서 제공하는 REST API들에 대해서 액션으로 정의하는 영역입니다.   

예를 들어, 위에서 설명했던 `cb-spider:` 서비스에서 제공하는 
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

**PathParam 사용법**   
`{driver_name}`처럼 {}로 지정된 액션의 경우에는, {}안에 있는 변수명이 매개변수 이름이되며, 사용자로부터 `-p` 또는 `--pathParam` 플래그를 이용해서 `--pathParam driver_name:AWS` 형태로 값을 전달 받을 수 있습니다.   
여러 개의 PathParam이 필요한 경우 공백으로 구분하여 `"key1:value1 key2:value2"` 형태로 지정합니다.

```
$ ./mayfly api --service cb-spider --action GetCloudDriver --pathParam driver_name:AWS

$ ./mayfly api -s cb-spider -a GetCloudDriver -p driver_name:AWS

$ ./mayfly api -s cb-spider -a GetCloudDriver -p driver_name:GCP
```

**QueryString 사용법**   
REST API에 쿼리 문자열이 필요한 경우 `-q` 또는 `--queryString` 플래그를 사용합니다.   
쿼리 문자열은 `param=value` 형태로 지정하며, 여러 개인 경우 `&`로 구분합니다.   
`?`는 자동으로 추가되므로 생략 가능하지만, 포함해도 정상 동작합니다.

```
# 단일 쿼리 문자열
$ ./mayfly api -s cb-spider -a GetRegionZone -p region_name:ap-northeast-3 -q ConnectionName=aws-config01
$ ./mayfly api -s cb-spider -a GetRegionZone -p region_name:ap-northeast-3 -q "?ConnectionName=aws-config01"

# 복수 쿼리 문자열 (&로 구분)
$ ./mayfly api -s cb-tumblebug -a Getmcivm -p "nsId:ns01 mciId:mci01 vmId:vm01" -q "option=status&accessInfo=showSshKey"
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

Cloud-Migrator 시스템의 경우 대부분의 프레임워크는 `Swagger` 기반으로 진행되고 있어서 Swagger 형태의 JSON 파일을 제공하고 있으므로, `api.yaml` 파일의 `serviceActions` 작성을 수월하게 하도록 `api tool` 서브 커맨드를 제공합니다. 유사한 환경의 다른 프레임워크 정보를 api.yaml에 정의하고 싶을 때 활용하시기 바랍니다.

`api tool`은 Swagger JSON을 파싱해 `serviceActions` 형태로 만들어 주며, 각 API의 액션 이름에는 내부적으로 `operationId` 필드를 사용하므로 `사용하려는 Swagger JSON에는 반드시 operationId 필드가 정의`되어 있어야 합니다(operationId가 없는 operation은 건너뜁니다).

**[Swagger JSON 파일 예시]**
```
"basePath": "/tumblebug",
"paths": {
	"/availableK8sClusterNodeImage": {
		"get": {
			"description": "Get available kubernetes cluster node image",
			"summary": "Get available kubernetes cluster node image",
			"operationId": "GetAvailableK8sClusterNodeImage",
```

### Swagger 소스 지정 (`-f` / `--latest` / `--release`)
Swagger 문서를 어디서 읽을지는 아래 세 옵션 중 **하나**로 지정합니다(동시에 두 개 이상 지정하면 오류).

- `-f <파일 또는 URL>` : 로컬 파일 경로 또는 `http(s)` URL을 직접 지정
- `--latest` : 각 서비스의 최신 Swagger URL 사용 (api.yaml의 `services.<svc>.swagger.latest`)
- `--release <tag>` : 특정 릴리스 태그의 Swagger 사용 (api.yaml의 `services.<svc>.swagger.release`, URL의 `{release}`가 태그로 치환됨. 예: `--release v0.5.2`)

`--latest`·`--release`는 api.yaml에 등록된 **Swagger URL 레지스트리**를 이용합니다. 각 서비스의 `services.<svc>.swagger` 항목에 아래처럼 latest/release URL이 정의되어 있습니다.
```
services:
  cb-spider:
    swagger:
      latest: https://raw.githubusercontent.com/cloud-barista/cb-spider/master/api/swagger.json
      release: https://raw.githubusercontent.com/cloud-barista/cb-spider/{release}/api/swagger.json
```
소스 옵션을 하나도 지정하지 않으면 최신/특정 릴리스 중 무엇을 쓸지 대화형으로 물어봅니다.

### 대상 서비스 지정 (`--service` / `--action`)
- `-f`로 단일 파일/URL을 줄 때는 **반드시 `--service <서비스명>`으로 대상 서비스를 지정**해야 합니다. 지정하지 않으면 `-f로 단일 소스를 줄 때는 --service로 대상 서비스를 지정하세요.` 오류가 납니다.
- `--action <액션명>`을 함께 주면 해당 서비스의 그 액션 **한 개만** 처리합니다(생략 시 서비스의 `serviceActions` 전체).
- `--latest`/`--release`에서 `--service`를 생략하면 특정 서비스만 처리할지, 레지스트리에 등록된 전체 서비스를 처리할지 대화형으로 선택합니다.

### 화면 출력 vs api.yaml 반영 (`--apply`)
- 기본 동작은 파싱 결과를 **화면에 출력**만 합니다(직접 복사해서 api.yaml을 작성).
- `--apply`를 주면 `conf/api.yaml`에 **직접 반영**합니다. 이때 `api.yaml.bak.<타임스탬프>` 백업을 먼저 만들고, 반영 후 결과가 YAML로 파싱되지 않으면 자동으로 원본을 복원합니다. 서비스 전체를 덤프하는 경우 `services.<svc>.version`도 Swagger의 버전으로 함께 갱신됩니다.

진행 직전에는 처리 대상 요약과 함께 `계속 진행하시겠습니까? (Y/n):` 확인을 거칩니다(Enter는 예). 자동화 시에는 `-y`(`--yes`)로 확인을 건너뛸 수 있으며, 이때는 소스와 대상(`--service`)을 반드시 명시해야 합니다.

**[사용 예시]**
```
# 로컬 파일을 cm-ant 대상으로 파싱해서 화면 출력
$ ./mayfly api tool -f ./cm-ant.swagger.json --service cm-ant

# 최신 URL 레지스트리에서 받아 api.yaml에 반영
$ ./mayfly api tool -f https://.../swagger.json --service cm-ant --apply

# 단일 액션만 갱신
$ ./mayfly api tool -f ./cm-ant.swagger.json --service cm-ant --action GetEstimateCost --apply

# 특정 릴리스 태그의 Swagger를 사용
$ ./mayfly api tool --release v0.5.2 --service cb-spider --apply
```

**[실행 결과 예시]**   
화면 출력 시 각 서비스마다 `# <서비스명>  (version=<버전>)` 헤더와 함께 `serviceActions` 본문이 출력됩니다.
```
$ ./mayfly api tool -f ./cb-tumblebug.json --service cb-tumblebug

# cb-tumblebug  (version=0.12.25)
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
```
위 출력 결과를 참고해서 api.yaml 파일을 작성하거나, `--apply`로 바로 반영하면 됩니다.


## 사용 방법
먼저 -s 또는 --service 플래그를 이용해서 호출을 희망하는 서비스(서브 시스템)를 지정하며, -a 또는 --action 플래그를 이용해서 해당 서비스에서 제공하는 API 중 호출하고 싶은 API를 지정합니다. 호출할 서비스 이름이나 액션 이름을 모를 경우 --list 플래그를 활용하시기 바랍니다.   

만약, 호출하려는 액션의 URI 경로가 가변 경로인 경우 --pathParam으로 경로 설정이 가능하며, 전달할 JSON Data 가 있는 경우 --data 또는 --file 플래그를 이용할 수 있습니다.   

아래는 사용 가능 커맨드와 플래그를 비롯한 일부 예시입니다.
```
Call the action of the service defined in api.yaml. For example:
$ ./mayfly api --help
$ ./mayfly api --list
$ ./mayfly api --service cb-spider --list
$ ./mayfly api --service cb-spider --action ListCloudOS
$ ./mayfly api --service cb-spider --action GetCloudDriver --pathParam driver_name:AWS
$ ./mayfly api --service cb-spider --action GetRegionZone --pathParam region_name:ap-northeast-3 --queryString ConnectionName=aws-config01
$ ./mayfly api --service cb-tumblebug --action Getmcivm --pathParam "nsId:ns01 mciId:mci01 vmId:vm01" --queryString "option=status&accessInfo=showSshKey"
$ ./mayfly api --service cm-beetle --action Deleteinfra --pathParam "nsId:mig01 mciId:mmci01" --queryString "option=terminate"

Usage:
  mayfly api [flags]
  mayfly api [command]

Available Commands:
  tool        Swagger JSON parsing into api.yaml serviceActions

Flags:
  -a, --action string        Action to perform
      --authPassword string  Password for basic authentication
      --authToken string     sets the auth token of the 'Authorization' header for all HTTP requests.(The default auth scheme is 'Bearer')
      --authUser string      Username for basic authentication
  -c, --config string        config file (default "./conf/api.yaml")
  -d, --data string          Data to send to the server
  -f, --file string          Data to send to the server from file
  -h, --help                 help for api
  -l, --list                 Show Service or Action list
  -o, --output string        <file> Write to file instead of stdout
  -p, --pathParam string     Variable path info set "key1:value1 key2:value2" for URIs (separated by space)
  -q, --queryString string   Query string to add to URIs (format: "param1=value1" or "param1=value1&param2=value2")
  -s, --service string       Service to perform
  -v, --verbose              Show more detail information

Use "mayfly api [command] --help" for more information about a command.
```

### 주요 flag 설명
--service (-s) : 호출할 서비스의 이름을 지정합니다.   
--action (-a) : 호출할 액션의 이름을 지정합니다.   
--pathParam (-p) : 경로 파라미터를 지정합니다. (형식: "key1:value1 key2:value2")   
--queryString (-q) : 쿼리 문자열을 지정합니다. (형식: "param1=value1" 또는 "param1=value1&param2=value2")   
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
쿼리 문자열은 "param1=value1" 형태로 지정하며, 여러 개인 경우 "&"로 구분합니다.
```
$ ./mayfly api --service <service_name> --action <action_name> --pathParam "key1:value1 key2:value2" --queryString "param1=value1&param2=value2"

$ ./mayfly api -s <service_name> -a <action_name> -p "key1:value1 key2:value2" -q "param1=value1&param2=value2"
```

### 환경 설정 파일과 실행 명령의 맵핑 관계
지금까지 설명드린 api.yaml에 정의되는 방식 및 CLI에서 호출하는 api 서브 커맨드의 맵핑 관계는 다음과 같습니다.
![apicall](https://github.com/cloud-barista/cm-mayfly/assets/78469943/17fe459b-cdab-402d-b87d-8f4ae12de646)


### 기본 사용 예시 
```
# 도움말 및 목록 조회
$ ./mayfly api --help
$ ./mayfly api --list
$ ./mayfly api --service cb-spider --list

# 단순 액션 호출
$ ./mayfly api --service cb-spider --action ListCloudOS

# PathParam 사용 (가변 경로)
$ ./mayfly api --service cb-spider --action GetCloudDriver --pathParam driver_name:AWS

# PathParam + QueryString 사용 (단일 쿼리)
$ ./mayfly api --service cb-spider --action GetRegionZone --pathParam region_name:ap-northeast-3 --queryString ConnectionName=aws-config01

# PathParam + QueryString 사용 (복수 쿼리 - &로 구분)
$ ./mayfly api --service cb-tumblebug --action Getmcivm --pathParam "nsId:ns01 mciId:mci01 vmId:vm01" --queryString "option=status&accessInfo=showSshKey"

# 마이그레이션된 인프라 삭제 (cm-beetle 사용 예시)
$ ./mayfly api --service cm-beetle --action Deleteinfra --pathParam "nsId:mig01 mciId:mmci01" --queryString "option=terminate"

# 또는 짧은 플래그 사용
$ ./mayfly api -s cb-tumblebug -a Getmcivm -p "nsId:ns01 mciId:mci01 vmId:vm01" -q "option=status&accessInfo=showSshKey"
$ ./mayfly api -s cm-beetle -a Deleteinfra -p "nsId:mig01 mciId:mmci01" -q "option=terminate"
```


### 실행 결과 예시
```
$ ./mayfly api --service cb-spider  --action ListCloudOS

service calling...
2024/05/02 09:39:33.922447 WARN RESTY Using Basic Auth in HTTP mode is not secure, use HTTPS
{"cloudos":["AWS","AZURE","GCP","ALIBABA","TENCENT","IBM","OPENSTACK","CLOUDIT","NCP","NCPVPC","NHNCLOUD","KTCLOUD","KTCLOUDVPC","DOCKER","MOCK","CLOUDTWIN"]}
```
