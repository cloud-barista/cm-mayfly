# 설치
mayfly CLI 도구를 설치하려면 다음 단계를 따르십시오:

1. GitHub 저장소에서 소스 코드를 다운로드하거나 클론합니다.
2. Go 언어로 빌드하거나 실행 가능한 파일을 다운로드합니다.
3. 실행 가능한 파일을 시스템 경로에 추가하여 mayfly를 전역으로 사용할 수 있도록 합니다.

## cm-mayfly 소스코드 다운로드

```bash
git clone https://github.com/cm-mayfly/cm-mayfly.git
```

# Golang 다운로드
```bash
wget https://go.dev/dl/go1.21.4.linux-amd64.tar.gz
```

## cm-mayfly 소스코드 빌드
```bash
cd cm-mayfly/src
go build -o mayfly main.go
```

## API 호출 관리 CLI사용 방법

이 CLI는 서비스 및 액션을 정의하고 해당 API를 호출하는데 사용됩니다.아래에서는 사용방법과 각 옵션에 대해 설명합니다.

## 설정

먼저 CLI를 실행가 전에 설정 파일을 준비해야 합니다.기본 설정 파일 경로는 "../conf/api.yaml"이지만, 필요에 따라 --config 옵션을 사용하여 다른 경로의 설정 파일을 지정할 수 있습니다.

## 기본 명령어 설명

--service (-s): 호출할 서비스의 이름을 지정합니다.
--action (-a): 호출할 액션의 이름을 지정합니다.
--list (-l): 서비스 목록 또는 액션 목록을 조회합니다.
--pathParam (-p): 경로 파라미터를 지정합니다.
--queryString (-q): 쿼리 문자열을 지정합니다.
--data (-d): 서버에 전송할 데이터를 지정합니다.
--file (-f): 서버에 전송할 데이터가 포함된 파일을 지정합니다.
--output (-o): 서버 응답을 파일에 저장합니다.

## 기본 명령어 예시 
./mayfly api --help
./mayfly api --list
./mayfly api --service mc-infra-connector --list
./mayfly api --service mc-infra-connector --action ListCloudOS
./mayfly api --service mc-infra-connector --action GetCloudDriver --pathParam driver_name:AWS
./mayfly api --service mc-infra-connector --action GetRegionZone --pathParam region_name:ap-northeast-3 --queryString ConnectionName:aws-config01

## 서비스 및 액션 선택

다음으로,호출할 서비스와 액션을 선택해야 합니다. 서비스와 액션을 선택하지 않고 CLI를 실행하면 서비스 및 액션의 목록이 표시 됩니다. 

특정 서비스의 특정 액션 호출:
./mayfly api --service <service_name> --action <action_name>

./mayfly api --service <service_name> --action <action_name> --pathParam <key1:value1 key2:value2> --queryString <query_string>

## 서비스 및 액션 예시
```
./mayfly api --service mc-infra-connector  --action ListCloudOS

service calling...
2024/05/02 09:39:33.922447 WARN RESTY Using Basic Auth in HTTP mode is not secure, use HTTPS
{"cloudos":["AWS","AZURE","GCP","ALIBABA","TENCENT","IBM","OPENSTACK","CLOUDIT","NCP","NCPVPC","NHNCLOUD","KTCLOUD","KTCLOUDVPC","DOCKER","MOCK","CLOUDTWIN"]}
```