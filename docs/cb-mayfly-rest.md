## Cloud-Migrator CLI REST API 호출 도구 (mayfly-rest)
Cloud-Migrator CLI API 호출 도구(mayfly)는 Cloud-Migrator 시스템의 RESTful API를 호출하는 간단한 명령 줄 인터페이스입니다. 이 도구를 사용하여 서비스와 액션을 호출하고 API 요청을 실행할 수 있습니다.

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

# 사용법 

mayfly-rest CLI 도구를 사용하여 REST API를 호출하려면 다음과 같이 명령을 사용합니다

mayfly-rest [flags]

# 예시
mayfly-rest --header "Content-Type: application/json" --data '{"key": "value"}'

## 구성 

mayfly-rest CLI 도구는 REST API 호출을 위한 구성을 사용합니다. 이 구성은 사용자 지정 헤더, 인증 정보 등을 포함하고 있습니다.

# 기능

사용자 지정 헤더 설정: --header 플래그를 사용하여 사용자 지정 헤더를 지정할 수 있습니다.
인증 정보 설정: --authToken 및 --authScheme 플래그를 사용하여 인증 토큰 및 스키마를 설정할 수 있습니다.
데이터 전송: --data 또는 --file 플래그를 사용하여 서버에 데이터를 전송할 수 있습니다.
출력 파일 설정: --output 플래그를 사용하여 응답을 파일로 저장할 수 있습니다.
상세 정보 출력: --verbose 플래그를 사용하여 상세 정보를 표시할 수 있습니다.
