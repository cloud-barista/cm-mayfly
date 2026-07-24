
## `cm-mayfly`의 `Docker Compose`를 이용한 Cloud-Migrator 설치 및 실행 가이드

이 가이드에서는 `cm-mayfly`의 `infra` 서브 커맨드를 이용하여 `Docker Compose` 기반으로 Cloud-Migrator 시스템을 구축 및 실행하는 방법에 대해 소개합니다. 


## 순서
- [`cm-mayfly`의 `Docker Compose`를 이용한 Cloud-Migrator 설치 및 실행 가이드](#cm-mayfly의-docker-compose를-이용한-cloud-migrator-설치-및-실행-가이드)
- [순서](#순서)
- [사전 준비 사항](#사전-준비-사항)
  - [Docker 설치](#docker-설치)
  - [Docker Compose 설치](#docker-compose-설치)
- [소스코드 다운로드](#소스코드-다운로드)
- [소스코드 빌드](#소스코드-빌드)
- [환경설정 확인 및 변경](#환경설정-확인-및-변경)
- [Cloud-Migrator 인프라 구축](#cloud-migrator-인프라-구축)
- [Cloud-Migrator 실행상태 확인](#cloud-migrator-실행상태-확인)
  - [기본 사용법](#기본-사용법)
  - [옵션 설명](#옵션-설명)
  - [사용 예시](#사용-예시)
  - [실행 결과 예시](#실행-결과-예시)
- [Cloud-Migrator 업데이트](#cloud-migrator-업데이트)
  - [기본 업데이트](#기본-업데이트)
  - [버전 체크 및 업데이트 확인](#버전-체크-및-업데이트-확인)
- [Cloud-Migrator 중지](#cloud-migrator-중지)
- [Cloud-Migrator 로그 조회](#cloud-migrator-로그-조회)
- [Cloud-Migrator 삭제(인프라 구축 환경 정리)](#cloud-migrator-삭제인프라-구축-환경-정리)
- [Docker 전체 환경 정리](#docker-전체-환경-정리)


## 사전 준비 사항
기본적인 내용은 README를 확인합니다.
### Docker 설치
- https://docs.docker.com/engine/install/ubuntu/ 에서 설명하는 방법대로 도커 환경을 설치합니다.

<details>
  <summary>[클릭하여 예시 보기]</summary>
  
```bash
# 기존에 Docker 가 설치되어 있었다면 삭제
$ sudo apt remove docker docker-engine docker.io containerd runc

# Docker 설치를 위한 APT repo 추가
$ sudo apt update

$ sudo apt install \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg \
    lsb-release

$ curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg

# x86_64 / amd64
$ echo \
  "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

$ sudo apt update

$ sudo apt install docker-ce docker-ce-cli containerd.io
```
</details>

### Docker Compose 설치
- APT 패키지 매니저를 이용하여 설치합니다.
```bash
$ sudo apt install docker-compose
```

## 소스코드 다운로드
```bash
$ git clone https://github.com/cloud-barista/cm-mayfly.git
```

## 소스코드 빌드
최상위 폴더에 빌드된 실행 파일이 함께 배포되며, 만약 소스 코드 빌드가 필요한 경우에는 README에 설명된 go 설치 방법과 make 명령의 빌드 방법을 참고합니다.
```bash
$ cd cm-mayfly
$ make
```

## 환경설정 확인 및 변경
Cloud-Migrator 시스템 구성에 필요한 정보는 `./conf/docker` 폴더 하위에 정의되어 있으니 시스템 구성전에 `./conf/docker/docker-compose.yaml` 파일 및 `./conf/docker/conf` 폴더의 내용들을 살펴 보고 필요한 경우 수정합니다.

CSP 자격증명(Credential) 등록은 인프라 구축 후 `mayfly setup credential` 명령으로 진행합니다. 자세한 내용은 [setup sub-command guide](./cm-mayfly-setup.md)를 참고하세요.


## Cloud-Migrator 인프라 구축
아래 명령을 실행하면 실행 대상 서비스가 카테고리별로 분류된 미리보기 표와 함께 출력되고, `Do you want to proceed with the installation? (y/N):` 확인을 거친 뒤 도커 기반 인프라 구축이 진행됩니다. (`y` 또는 `yes` 외의 입력은 취소로 처리됩니다.)
```bash
$ ./mayfly infra run
```

전체 스택(서비스 미지정)으로 실행하는 경우, 다른 서비스를 기동하기 전에 OpenBao 상태 정합성 preflight가 먼저 수행됩니다. `.env`에 `VAULT_TOKEN`이 없는 최초 구축이면 OpenBao를 자동으로 초기화(`VAULT_TOKEN` 기록)한 뒤 나머지 서비스를 올리고, 상태가 일치하면 그대로 진행합니다. 토큰이 남아 있으나 저장소와 어긋나는 등 불일치가 감지되면 안내 메시지를 출력하고 **나머지 서비스를 기동하지 않은 채 중단**하므로, 안내에 따라 조치한 뒤 다시 실행하세요. OpenBao 운영에 대한 자세한 내용은 [openbao-unseal.md](./openbao-unseal.md)를 참고하세요.

`-s`로 특정 서비스만 부분 기동하는 경우에는 OpenBao를 자동 초기화하지 않습니다. 다만 대상 서비스가 OpenBao를 사용하면(compose에서 `VAULT_*`를 참조 — 예: cb-tumblebug·mc-terrarium) **읽기 전용으로 OpenBao 상태를 먼저 확인**해, 사용할 수 없는 상태면 `mayfly setup openbao init`(OpenBao만 초기화) 또는 전체 `mayfly infra run`을 안내하고 **기동하지 않고 중단**합니다(초기화되지 않은 OpenBao에 붙어 첫 시크릿 조회에서 실패하는 것을 막습니다). OpenBao를 사용하지 않는 서비스는 이 검사를 거치지 않습니다. `infra update -s`도 동일하게 동작합니다.

만약, Cloud-Migrator 시스템을 구축하려는 시스템 환경이 Clean한 환경이 아니라서 `./mayfly infra run` 명령만으로는 제대로 실행되지 않는 설치 문제가 발생할 경우에는 [Docker 전체 환경 정리](#docker-전체-환경-정리) 섹션의 내용을 확인해서 시스템 환경을 먼저 깔끔하게 정리 후 실행하는 것을 추천드립니다.   


설치 과정을 보고 싶지 않다면 -d 옵션이나 --detach 플래그를 사용해서 설치 과정을 백그라운드로 실행할 수 있습니다.
```bash
$ ./mayfly infra run -d
```


특정 프레임워크만 실행하고 싶으면 아래처럼 실행합니다.   
예를 들어, cb-tumblebug을 실행하고 싶은 경우..
```bash
$ ./mayfly infra run -s cb-tumblebug
```

여러 프레임워크를 동시에 실행하고 싶은 경우 — 아래 세 형식은 모두 같은 서비스를 선택합니다:
```bash
# -s 를 반복해서 지정
$ ./mayfly infra run -s cb-tumblebug -s cb-spider

# 공백으로 구분
$ ./mayfly infra run -s "cb-tumblebug cb-spider"

# 콤마로 구분
$ ./mayfly infra run -s "cb-tumblebug,cb-spider"
```

> `-s` 를 아예 생략하면 전체 서비스가 대상입니다. 존재하지 않는 서비스명을 지정하면 그 이름을 지목해 알리고 중단합니다.

## Cloud-Migrator 실행상태 확인
설치된 인프라 및 서브 프레임워크들의 상태를 확인할 수 있습니다.

### 기본 사용법
```bash
$ ./mayfly infra info
```

### 옵션 설명
- `-a, --all`: 모든 컨테이너 상태 표시 (실행 중인 컨테이너뿐만 아니라 중지된 컨테이너도 포함)
  - **주의**: 완전히 삭제된 컨테이너는 표시되지 않습니다
- `-s, --service`: 특정 서비스만 대상으로 지정 (여러 서비스 지정 가능)
  - **지원 형식**: `-s` 반복 지정, 공백 구분, 콤마 구분 (혼용 가능)
  - **예시**: `-s cb-tumblebug -s cb-spider` 또는 `-s "cb-tumblebug cb-spider"` 또는 `-s "cb-tumblebug,cb-spider"`
  - **의존성 자동 포함**: 지정된 서비스의 의존성 서비스들도 함께 표시  
- `-u, --human`: 인간이 이해하기 쉬운 서비스 상태 테이블 표시
  - **특징**: docker-compose.yaml에 정의된 모든 서비스의 상태를 표 형태로 표시
  - **표시 항목**: 서비스명, 버전, 상태, 헬스 상태, 내부 포트, 외부 포트, 이미지 크기
  - **서비스 분류**: 요청된 서비스와 의존성 서비스를 구분하여 표시
- `-t, --test-versions`: 서비스명과 버전 추출 테스트 및 디버깅용 정보 표시
  - **목적**: docker-compose.yaml와 실행 중인 서비스의 버전 정보 추출 테스트 및 서비스 실행 상태 비교
  - **표시 항목**: 서비스명, docker-compose.yaml 버전, 실행 상태, 실제 이미지 버전, 이미지 크기
  - **디버깅**: -u 옵션의 동작 테스트를 겸하고 있으며 docker-compose.yaml과 실제 실행 중인 서비스로 항목이 구분되어 있어서 -u 옵션 보다 조금 더 현재 상태 파악이 직관적일 수도 있음.

### 사용 예시
```bash
# 실행 중인 컨테이너만 표시 (기본값)
$ ./mayfly infra info

# 모든 컨테이너 상태 표시 (중지된 컨테이너 포함)
$ ./mayfly infra info -a
$ ./mayfly infra info --all

# 특정 서비스만 표시
$ ./mayfly infra info -s cb-tumblebug

# 여러 서비스 표시 (공백으로 구분)
$ ./mayfly infra info -s "cb-tumblebug cb-spider"

# 여러 서비스 표시 (콤마로 구분)
$ ./mayfly infra info -s "cb-tumblebug,cb-spider"

# 인간이 이해하기 쉬운 서비스 상태 테이블 표시
$ ./mayfly infra info -u
$ ./mayfly infra info --human

# 특정 서비스와 의존성을 테이블 형태로 표시
$ ./mayfly infra info -s cb-tumblebug -u

# 여러 서비스와 의존성을 테이블 형태로 표시
$ ./mayfly infra info -s "cb-tumblebug cm-ant" -u

# 버전 추출 테스트 및 디버깅 정보 표시
$ ./mayfly infra info -t
$ ./mayfly infra info --test-versions
```

### 실행 결과 예시

#### 기본 info 명령
```bash
$ ./mayfly infra info
```
```
[Get info for Cloud-Migrator runtimes]

[v]Status of Cloud-Migrator runtimes
NAME             IMAGE                                  COMMAND                   SERVICE          CREATED      STATUS                PORTS
airflow-mysql    mysql:8.0-debian                       "docker-entrypoint.s…"    airflow-mysql    4 days ago   Up 4 days             0.0.0.0:3306->3306/tcp, :::3306->3306/tcp, 33060/tcp
airflow-redis    redis:7.2-alpine                       "docker-entrypoint.s…"    airflow-redis    8 days ago   Up 4 days (healthy)   0.0.0.0:6379->6379/tcp, :::6379->6379/tcp
cb-spider        cloudbaristaorg/cb-spider:edge         "/root/go/src/github…"    cb-spider        8 days ago   Up 4 days (healthy)   0.0.0.0:1024->1024/tcp, 0.0.0.0:2048->2048/tcp
cb-tumblebug     cloudbaristaorg/cb-tumblebug:edge      "/app/src/cb-tumbleb…"    cb-tumblebug     8 days ago   Up 4 days (healthy)   0.0.0.0:1323->1323/tcp
cm-ant           cloudbaristaorg/cm-ant:edge            "./ant"                   cm-ant           8 days ago   Up 4 days (healthy)   0.0.0.0:8880->8880/tcp, :::8880->8880/tcp

[v]Status of Cloud-Migrator runtime images
CONTAINER           REPOSITORY                       TAG                 IMAGE ID            SIZE
airflow-mysql       mysql                            8.0-debian          ccb4819cef05        611MB
airflow-redis       redis                            7.2-alpine          97ed3031282d        40.7MB
cb-spider           cloudbaristaorg/cb-spider        edge                b241e15bba26        386MB
cb-tumblebug        cloudbaristaorg/cb-tumblebug     edge                101876d9e57f        117MB
cm-ant              cloudbaristaorg/cm-ant           edge                9691839034bf        178MB
```

#### --human 옵션 사용 (전체 서비스)
```bash
$ ./mayfly infra info --human
```
```
[Cloud-Migrator Service Status]

┌───────────────────────┬──────────────┬──────────────┬──────────┬──────────────┬──────────────┬─────────────────┐
│SERVICE                │VERSION       │STATUS        │HEALTHY   │INTERNAL      │EXTERNAL      │IMAGE SIZE       │
├───────────────────────┼──────────────┼──────────────┼──────────┼──────────────┼──────────────┼─────────────────┤
│cb-spider              │0.12.35       │running       │✓         │1024          │1024          │436MB            │
│cb-tumblebug           │0.12.25       │running       │✓         │1323          │1323          │146MB            │
│cb-tumblebug-etcd      │v3.6.11       │running       │✓         │2379-2380     │2379-2380     │60.4MB           │
│cb-tumblebug-postgres  │16-alpine     │running       │✓         │5432          │6432          │281MB            │
│cb-mapui               │0.12.50       │running       │✓         │1324          │1324          │422MB            │
│cm-beetle              │0.5.6         │running       │✓         │8056          │8056          │138MB            │
│cm-butterfly-api       │0.5.1         │running       │✓         │4000          │4000          │94.4MB           │
│cm-butterfly-front     │0.5.1         │running       │✓         │80            │80            │54.6MB           │
│cm-butterfly-db        │14-alpine     │running       │✓         │5432          │543           │278MB            │
│cm-honeybee            │0.5.3         │running       │✓         │8081          │8081          │56.2MB           │
│cm-damselfly           │0.5.3         │running       │✓         │8088          │8088          │100MB            │
│cm-cicada              │0.5.2         │running       │✓         │8083          │8083          │890MB            │
│airflow-redis          │7.2-alpine    │running       │✓         │6379          │6379          │40.9MB           │
│airflow-mysql          │8.0-debian    │running       │✓         │3306          │3306          │610MB            │
│airflow-server         │0.5.2         │running       │✓         │5555          │5555          │1.57GB           │
│cm-grasshopper         │0.5.2         │running       │✓         │8084          │8084          │448MB            │
│cm-ant                 │0.5.4         │running       │✓         │8880          │8880          │192MB            │
│ant-postgres           │latest-pg16   │running       │✓         │5432          │5432          │1.04GB           │
└───────────────────────┴──────────────┴──────────────┴──────────┴──────────────┴──────────────┴─────────────────┘
```

#### --human 옵션 사용 (특정 서비스 + 의존성)
```bash
$ ./mayfly infra info -s cb-tumblebug --human
```
```
[Cloud-Migrator Service Status]

🎯 Requested Services:
┌──────────────────────┬──────────────┬──────────────┬──────────┬──────────────┬──────────────┬─────────────────┐
│SERVICE               │VERSION       │STATUS        │HEALTHY   │INTERNAL      │EXTERNAL      │IMAGE SIZE       │
├──────────────────────┼──────────────┼──────────────┼──────────┼──────────────┼──────────────┼─────────────────┤
│cb-tumblebug          │0.12.25       │running       │✓         │1323          │1323          │146MB            │
└──────────────────────┴──────────────┴──────────────┴──────────┴──────────────┴──────────────┴─────────────────┘

📦 Dependency Services:
┌───────────────────────┬──────────────┬──────────────┬──────────┬──────────────┬──────────────┬─────────────────┐
│SERVICE                │VERSION       │STATUS        │HEALTHY   │INTERNAL      │EXTERNAL      │IMAGE SIZE       │
├───────────────────────┼──────────────┼──────────────┼──────────┼──────────────┼──────────────┼─────────────────┤
│cb-tumblebug-etcd      │v3.6.11       │running       │✓         │2379-2380     │2379-2380     │60.4MB           │
│cb-spider              │0.12.35       │running       │✓         │1024          │1024          │436MB            │
│cb-tumblebug-postgres  │16-alpine     │running       │✓         │5432          │6432          │281MB            │
└───────────────────────┴──────────────┴──────────────┴──────────┴──────────────┴──────────────┴─────────────────┘
```

#### --test-versions 옵션 사용
```bash
$ ./mayfly infra info --test-versions
```
```
=== Version Extraction Test & Service Status ===

SERVICE              COMPOSE_VERSION STATUS       ACTUAL_VERSION  IMAGE_SIZE
--------------------------------------------------------------------------------
cb-spider            0.12.35         running      0.12.35         436MB
cb-tumblebug         0.12.25         running      0.12.25         146MB
cb-tumblebug-etcd    v3.6.11         running      v3.6.11         60.4MB
cb-tumblebug-postgres 16-alpine       running      16-alpine       281MB
cb-mapui             0.12.50         running      0.12.50         422MB
cm-beetle            0.5.6           running      0.5.6           137MB
cm-butterfly-api     0.5.1           running      0.5.1           94.4MB
cm-butterfly-front   0.5.1           Not Running  -               -
cm-butterfly-db      14-alpine       running      14-alpine       278MB
cm-honeybee          0.5.3           running      0.5.3           56.2MB
cm-damselfly         0.5.3           running      0.5.3           100MB
cm-cicada            0.5.2           running      0.5.2           890MB
airflow-redis        7.2-alpine      running      7.2-alpine      40.9MB
airflow-mysql        8.0-debian      running      8.0-debian      610MB
airflow-server       0.5.2 (Not Downloaded) running      0.5.1           1.57GB
cm-grasshopper       0.5.2           running      0.5.2           448MB
cm-ant               0.5.4           running      0.5.4           193MB
ant-postgres         latest-pg16     running      latest-pg16     1.07GB

Legend:
  COMPOSE_VERSION: Version specified in docker-compose.yaml
  STATUS: Current container status (running/stopped/Not Running)
  ACTUAL_VERSION: Version from running container image tag
  IMAGE_SIZE: Size of the container image
===============================================
```

**--test-versions 옵션의 장점:**
- **버전 비교**: docker-compose.yaml에 정의된 버전과 실제 실행 중인 이미지 버전을 한눈에 비교
- **디버깅**: 버전 정보 추출 문제나 서비스 상태 불일치 버그를 쉽게 파악
- **상태 분석**: 실행 중이 아닌 서비스("Not Running")를 명확히 식별
- **이미지 관리**: 로컬에 다운로드되지 않은 이미지("Not Downloaded") 확인 가능
- **버전 불일치 감지**: docker-compose.yaml 버전과 실제 실행 버전이 다른 경우를 쉽게 발견
  - `(Not Downloaded)` 표시는 docker-compose.yaml에 정의된 이미지가 로컬에 없을 때 나타남
  - 이 경우 실제로는 다른 버전의 이미지로 컨테이너가 실행 중일 수 있음 (예: airflow-server의 경우 0.5.2가 정의되어 있지만 0.5.1로 실행 중)

**--human 옵션의 장점:**
- **직관적**: docker-compose.yaml에 정의된 모든 서비스가 한눈에 보임
- **구조화**: 표 형태로 정보가 정리되어 읽기 쉬움
- **상태 파악**: 각 서비스의 실행 상태와 헬스 상태를 명확히 확인 가능
- **포트 정보**: 내부/외부 포트가 분리되어 표시됨
- **버전 정보**: 각 서비스의 버전을 한눈에 확인 가능
- **의존성 파악**: `-s` 옵션과 함께 사용 시 서비스 간 의존성 관계를 명확히 표시
- **서비스 분류**: 요청된 서비스와 의존성 서비스를 구분하여 표시
- **유연한 서비스 지정**: `-s` 반복 지정·공백·콤마로 여러 서비스를 동시에 지정 가능


## Cloud-Migrator 업데이트
Cloud-Migrator 서브 시스템들의 최신 버전으로 업데이트하고 싶은 경우 update 명령으로 현재 환경을 최신 버전으로 재구축할 수 있습니다.

### 기본 업데이트
```bash
$ ./mayfly infra update
```

업데이트가 끝나면 기본적으로 컨테이너 로그를 이어서 출력합니다(따라가기). 로그를 보지 않고 백그라운드로만 재기동하고 싶으면 `-d` 또는 `--detach` 플래그를 사용하세요.
```bash
$ ./mayfly infra update -d
```

### 버전 체크 및 업데이트 확인
`mayfly infra update` 명령은 업데이트 전에 각 서비스의 버전 상태를 체크하고 사용자에게 확인을 요청합니다.

#### 동작 방식
1. **로컬 이미지 버전 확인**: 실행 중인 컨테이너가 쓰는 이미지 태그
2. **docker-compose.yaml 버전 확인**: 설정 파일에 정의된 태그
3. **Docker Hub 조회**: 해당 태그를 언제 마지막으로 올렸는지(`Hub updated`)와 그 태그가 현재 가리키는 내용(다이제스트)
4. **버전 비교 및 표시**: 각 서비스별 상태를 테이블로 표시
5. **사용자 확인**: 갱신이 필요한 경우 대상 서비스를 알리고 확인 요청

**태그 이름이 같아도 내용이 바뀌었으면 갱신 대상입니다.** `edge`·`latest`처럼 움직이는 태그는 새로 빌드돼도 이름이 그대로라 이름 비교만으로는 절대 잡히지 않습니다. 그래서 이름이 같은 경우에는 로컬 이미지와 Docker Hub의 다이제스트를 대조합니다.

#### 출력 예시
```
🔍 Checking version updates...

📊 Version Comparison:
┌────────────────────┬───────────────┬─────────┬─────────────┐
│ Service            │ Local         │ Compose │ Hub updated │
├────────────────────┼───────────────┼─────────┼─────────────┤
│ cb-spider          │ not_installed │ 0.12.35 │ 2026-06-30  │ ✗
│ cb-tumblebug       │ 0.12.25       │ 0.12.25 │ 2026-07-02  │ ✓
│ cm-ant             │ 0.5.4         │ 0.5.7   │ 2026-07-21  │ ●
│ cm-butterfly-front │ edge          │ edge    │ 2026-07-24  │ ◆
└────────────────────┴───────────────┴─────────┴─────────────┘

Legend:
✓ Up to date
● Tag differs from docker-compose.yaml (update needed)
◆ Same tag, but Docker Hub holds different content (update needed)
✗ Image not installed locally
? Local version could not be read — left out of the update

Hub updated is when Docker Hub last pushed the tag docker-compose.yaml asks for.
It is information, not the verdict: the verdict comes from the columns to its left.

Details:
  ✗ cb-spider — not installed locally
  ● cm-ant — tag differs from docker-compose.yaml
  ◆ cm-butterfly-front — same tag, but Docker Hub holds different content

➡️  3 service(s) will be updated: cb-spider, cm-ant, cm-butterfly-front
   Everything else keeps running untouched.
Do you want to proceed with the update? (y/N): y
```

#### 갱신 범위

**`-s` 없이 실행하면 위 표에서 갱신이 필요하다고 판정된 서비스만** 내려받고 재기동합니다. 변경이 없는 서비스는 실행 중인 상태 그대로 둡니다.

`-s`를 주면 그 지정이 우선합니다. 서비스를 직접 고르는 것은 사용자의 결정이므로 버전 판정이 그것을 덮어쓰지 않습니다.

갱신 대상이 하나도 없을 때는 "모두 최신"이라고 알린 뒤, 그래도 내려받아 재기동할지 따로 묻습니다. 이 경우에는 좁힐 근거가 없으므로 전체가 대상입니다.

> **판단이 불확실하면 건드리지 않습니다.** Docker Hub 조회가 실패했거나 로컬 이미지에 다이제스트가 없어(직접 빌드한 이미지 등) 비교할 수 없으면 그 서비스는 갱신 대상에서 제외합니다. 로컬 버전을 읽지 못한 서비스도 마찬가지입니다(`?` 표시). 네트워크가 잠깐 끊겼다는 이유로 환경 전체가 재기동되면 안 되기 때문입니다.

#### 사용법
```bash
# 갱신이 필요한 서비스만 업데이트 (버전 체크 포함)
$ ./mayfly infra update

# 특정 서비스만 업데이트 (버전 체크 포함)
$ ./mayfly infra update -s cm-ant
$ ./mayfly infra update -s cb-tumblebug

# 여러 서비스를 동시에 업데이트
$ ./mayfly infra update -s "cb-tumblebug cb-spider"
$ ./mayfly infra update -s "cm-ant,cm-cicada"
$ ./mayfly infra update -s cm-ant -s cm-cicada
```

#### 특징
- **docker-compose.yaml 기준**: 설정 파일에 정의된 버전으로 로컬 환경 맞춤
- **바뀐 것만 갱신**: 판정된 서비스만 pull·재기동하고 나머지는 건드리지 않음
- **움직이는 태그 대응**: 태그 이름이 같아도 내용이 바뀌었으면 다이제스트로 탐지
- **사용자 친화적**: 업데이트 전 대상과 근거를 함께 제시
- **안전한 업데이트**: 사용자 확인 후에만 업데이트 진행
- **호환성 보장**: docker-compose.yaml에 정의된 버전 우선 사용



## Cloud-Migrator 중지
일부 또는 전체 프레임워크를 잠시 중지할 때 사용합니다.
```bash
$ ./mayfly infra stop
```


특정 프레임워크만 중지하고 싶으면 아래처럼 실행합니다.   
예를 들어, cb-tumblebug을 중지하고 싶은 경우..
```bash
$ ./mayfly infra stop -s cb-tumblebug
```

여러 프레임워크를 동시에 중지하고 싶은 경우:
```bash
# 공백으로 구분
$ ./mayfly infra stop -s "cb-tumblebug cb-spider"

# 콤마로 구분 (자동으로 공백으로 변환됨)
$ ./mayfly infra stop -s "cb-tumblebug,cb-spider"
```

중지 후 표시되는 컨테이너 상태 목록에 중지된 컨테이너까지 포함해서 보고 싶으면 `-a` 또는 `--all` 플래그를 사용합니다.
```bash
$ ./mayfly infra stop -a
```


## Cloud-Migrator 로그 조회

Cloud-Migrator 시스템의 로그를 조회할 수 있습니다. 다양한 옵션을 통해 효율적으로 로그를 확인할 수 있습니다.

### 기본 사용법
```bash
# 마지막 10줄 로그를 출력하고 실시간으로 따라가기 (기본값)
$ ./mayfly infra logs

# 마지막 10줄 로그만 출력하고 종료
$ ./mayfly infra logs --no-follow
```
기본적으로는 마지막 10줄 로그를 출력하고 실시간으로 모니터링합니다. 로그를 확인하고 바로 종료하려면 `--no-follow` 옵션을 사용하세요.

### 옵션 설명

| 옵션 | 설명 | 예시 |
|------|------|------|
| `-s, --service` | 특정 서비스만 대상으로 지정 (여러 서비스는 공백 또는 콤마로 구분) | `-s cb-tumblebug` / `-s "cb-tumblebug cb-spider"` |
| `-n, --tail` | 마지막 N줄부터 출력 (기본값 10, 전체 로그는 `all`) | `--tail 50` / `-n 50` |
| `--since` | 특정 시간 이후의 로그만 출력 | `--since 1h` |
| `--follow` | 실시간으로 로그를 따라가기 (기본값: true) | `--follow` |
| `--no-follow` | follow 모드 비활성화 (로그 확인 후 종료) | `--no-follow` |

> `--tail`의 단축키는 `-n`입니다(`-t`가 아닙니다). 전체 로그를 처음부터 보려면 `--tail all`을 사용하세요. `--tail 0`은 0줄을 의미하므로 "모든 로그"가 아닙니다.


### 서비스 이름
`-s`로 지정하는 서비스 이름은 `docker-compose.yaml`에 정의된 이름과 정확히 일치해야 합니다(잘못된 이름을 주면 사용 가능한 목록과 함께 오류로 거부됩니다). 예를 들어 CM-Butterfly는 단일 `cm-butterfly`가 아니라 `cm-butterfly-api`·`cm-butterfly-front`·`cm-butterfly-db`로 나뉘어 있습니다. 현재 라인업의 서비스 이름은 다음과 같습니다(총 22개).

- `cb-spider`: CB-Spider (멀티 클라우드 연동)
- `cb-tumblebug`: CB-Tumblebug (멀티 클라우드 인프라 관리)
- `cb-tumblebug-etcd`: CB-Tumblebug etcd
- `cb-tumblebug-postgres`: CB-Tumblebug PostgreSQL
- `cb-mapui`: CB-MapUI
- `mc-terrarium`: MC-Terrarium
- `openbao`: OpenBao (시크릿 관리)
- `openbao-unseal`: OpenBao unseal 사이드카
- `cm-beetle`: CM-Beetle
- `cm-butterfly-api`: CM-Butterfly API (백엔드)
- `cm-butterfly-front`: CM-Butterfly Front (웹 콘솔)
- `cm-butterfly-db`: CM-Butterfly DB
- `cm-honeybee`: CM-Honeybee
- `cm-damselfly`: CM-Damselfly
- `cm-cicada`: CM-Cicada
- `airflow-redis`: Airflow Redis
- `airflow-mysql`: Airflow MySQL
- `airflow-server`: Airflow Server
- `cm-grasshopper`: CM-Grasshopper
- `cm-grasshopper-rustfs`: CM-Grasshopper RustFS (오브젝트 스토리지)
- `cm-ant`: CM-Ant
- `ant-postgres`: CM-Ant PostgreSQL

> 정확한 최신 목록은 `./mayfly infra info --human`으로도 확인할 수 있습니다.


### 사용 예시

#### 1. 모든 서비스 로그 조회
```bash
# 마지막 10줄부터 실시간 follow (기본값)
$ ./mayfly infra logs

# 마지막 10줄만 출력하고 종료
$ ./mayfly infra logs --no-follow

# 마지막 50줄부터 실시간 follow
$ ./mayfly infra logs --tail 50

# 마지막 50줄만 출력하고 종료
$ ./mayfly infra logs --tail 50 --no-follow

# 처음부터 모든 로그 출력하고 실시간 follow
$ ./mayfly infra logs --tail all

# 1시간 전부터의 로그를 실시간 follow
$ ./mayfly infra logs --since 1h
```

#### 2. 특정 서비스 로그 조회
```bash
# cb-tumblebug 서비스의 로그만 조회
$ ./mayfly infra logs -s cb-tumblebug

# cm-ant 서비스의 마지막 20줄 로그
$ ./mayfly infra logs -s cm-ant --tail 20

# cb-tumblebug 서비스의 마지막 50줄 로그
$ ./mayfly infra logs -s cb-tumblebug --tail 50

# cm-ant 서비스의 1시간 전부터 마지막 20줄 로그
$ ./mayfly infra logs -s cm-ant --tail 20 --since 1h
```

#### 3. 시간 기반 로그 조회
```bash
# 30분 전부터의 로그
$ ./mayfly infra logs --since 30m

# 2시간 전부터의 로그
$ ./mayfly infra logs --since 2h

# 특정 시간 이후의 로그 (ISO 8601 형식)
$ ./mayfly infra logs --since 2024-01-15T10:30:00
```

#### 4. 조합 사용 예시
```bash
# cb-tumblebug 서비스의 마지막 30줄 + 30분 전부터
$ ./mayfly infra logs -s cb-tumblebug --tail 30 --since 30m

# cm-butterfly-front 서비스의 마지막 100줄 + 2시간 전부터
$ ./mayfly infra logs -s cm-butterfly-front --tail 100 --since 2h
```

### 로그 필터링 (grep 활용)

특정 키워드가 포함된 로그만 필터링하여 조회할 수 있습니다.

#### 기본 필터링
```bash
# ERROR가 포함된 로그만 출력
./mayfly infra logs | grep ERROR

# WARN이 포함된 로그만 출력
./mayfly infra logs | grep WARN

# 특정 서비스의 ERROR 로그만 출력
./mayfly infra logs -s cb-tumblebug | grep ERROR
```

#### 고급 필터링 옵션
```bash
# 대소문자 구분 없이 error 검색
./mayfly infra logs | grep -i error

# ERROR 또는 WARN이 포함된 로그 출력
./mayfly infra logs | grep -E "ERROR|WARN"

# ERROR가 포함된 로그와 그 앞뒤 2줄도 함께 출력
./mayfly infra logs | grep -A 2 -B 2 ERROR

# 마지막 50줄에서 ERROR만 검색
./mayfly infra logs --tail 50 | grep ERROR
```

#### 실용적인 조합 예시
```bash
# cb-tumblebug 서비스의 마지막 100줄에서 ERROR만 검색
./mayfly infra logs -s cb-tumblebug --tail 100 | grep ERROR

# 1시간 전부터 ERROR나 WARN이 포함된 로그만 출력
./mayfly infra logs --since 1h | grep -E "ERROR|WARN"

# cm-ant 서비스의 ERROR 로그와 컨텍스트 함께 출력
./mayfly infra logs -s cm-ant | grep -A 3 -B 3 ERROR
```

**주의사항**: `grep`을 사용하면 `--follow` 옵션의 실시간 로그 스트리밍이 중단될 수 있습니다. 실시간으로 특정 키워드의 로그만 보고 싶다면 별도의 터미널에서 실행하거나, 로그 파일을 직접 모니터링하는 방법을 고려해보세요.

### 팁
- **성능 최적화**: 오래된 로그로 인한 지연을 방지하려면 `--tail` 옵션을 사용하세요
- **디버깅**: 특정 서비스의 문제를 해결할 때는 `-s` 옵션으로 해당 서비스만 조회하세요
- **시간 기반 분석**: 특정 시간대의 문제를 분석할 때는 `--since` 옵션을 활용하세요
- **로그 필터링**: `grep` 명령어를 활용하여 원하는 키워드가 포함된 로그만 효율적으로 조회하세요



## Cloud-Migrator 삭제(인프라 구축 환경 정리)
더 이상 Cloud-Migrator 인프라가 필요 없거나 새로 구축하고 싶을 경우에는 아래와 같은 방법으로 정리가 가능합니다.

### 전체 시스템 제거

기본적으로 컨테이너와 프로젝트 네트워크만 삭제하며, 이미지와 호스트 데이터(`conf/docker/data/`)는 보존합니다(`docker compose down`과 동일). 데이터가 남으므로 `mayfly infra run`으로 다시 올리면 기존 상태 그대로 복귀합니다.
```bash
$ ./mayfly infra remove
```

이미지와 호스트 데이터(`conf/docker/data/*` 중 openbao 제외)까지 함께 삭제합니다. OpenBao 자격증명(데이터·`.env` 토큰)은 보존됩니다.
```bash
$ ./mayfly infra remove --clean-db
```

`--clean-db`가 삭제하는 모든 것에 더해 OpenBao 호스트 데이터까지 삭제하고 `.env`의 `VAULT_TOKEN`을 비웁니다.   
완전히 최초 상태로 재구축하고 싶을 때 사용하세요. 이후 재구동 시 OpenBao가 자동으로 재초기화됩니다.
```bash
$ ./mayfly infra remove --clean-all
```

> 참고: 확인 프롬프트를 건너뛰려면 `-y`, 실제 실행 없이 수행될 명령만 미리 보려면 `--dry-run`을 사용합니다. `-s`(특정 서비스)는 `--clean-all`과 함께 사용할 수 없습니다.

### 특정 서비스 제거

특정 서비스만 제거하고 싶은 경우 `-s` 옵션을 사용합니다. 이 방식은 Docker 네트워크를 보존하여 다른 서비스들의 연결성을 유지합니다.

```bash
# 특정 서비스 제거 (네트워크 보존)
$ ./mayfly infra remove -s cb-tumblebug

# 여러 서비스 제거 (공백으로 구분)
$ ./mayfly infra remove -s "cb-tumblebug cb-spider"

# 여러 서비스 제거 (콤마로 구분)
$ ./mayfly infra remove -s "cb-tumblebug,cb-spider"
```

특정 서비스 제거 시 이미지·데이터까지 함께 정리:
```bash
# 특정 서비스 + 해당 서비스의 이미지 + 호스트 데이터(conf/docker/data/<서비스>)까지 제거
$ ./mayfly infra remove -s cb-tumblebug --clean-db
```

`--clean-db`는 `-s` 유무와 무관하게 **대상 서비스의 이미지를 삭제**합니다. 이미지가 남아 있으면 `docker compose up`이 로컬 이미지를 그대로 재사용하기 때문에, `edge`·`latest`처럼 태그가 이동하는 이미지에서는 *삭제 후 다시 받았는데도 예전 버전이 뜨는* 현상이 생깁니다. 이미지를 지우면 다음 `mayfly infra run`이 반드시 새로 받습니다.

이미지를 그대로 두고 컨테이너만 재기동하고 싶다면 `--clean-db` 없이 실행하세요(재다운로드 없음).

> [!NOTE]
> - 특정 서비스 제거 시 네트워크가 보존되어 다른 서비스에 영향을 주지 않습니다.
> - `-s`(특정 서비스)는 `--clean-all`과 함께 사용할 수 없습니다(의도가 모호). OpenBao만 재초기화하려면 `mayfly infra remove -s openbao --clean-db`, 전체 환경 + OpenBao는 `mayfly infra remove --clean-all`을 사용하세요.

**참고**: `./conf/docker/data` 폴더 하위에 각 서브 프레임워크(컨테이너) 이름의 폴더가 생성되어 Data·Log 등을 보관합니다.   
`--clean-db`/`--clean-all`은 이 호스트 데이터 폴더까지 정리하며(위 규칙대로 openbao 예외), 플래그 없이 `remove`만 실행하면 컨테이너만 삭제하고 `./conf/docker/data`는 보존합니다.



## Docker 전체 환경 정리
Cloud-Migrator 인프라를 구축하기 전에 사용했던 도커 환경과의 충돌로 인해 모든 환경을 초기화하고 싶은 경우 아래 명령어로 초기화가 가능합니다.   
myfly로 구축한 도커 환경 외에도 `시스템에 존재하는 모든 도커 환경이 삭제`됩니다.

> [!CAUTION]
> **위험: 시스템의 모든 Docker 리소스가 삭제됩니다!**
> 
> 아래 명령어들은 다음을 포함한 **모든 Docker 관련 데이터**를 제거합니다:
> 
> - **모든 Docker 컨테이너** (실행 중 및 중지됨)
> - **모든 Docker 이미지**
> - **모든 Docker 볼륨**
> - **모든 커스텀 Docker 네트워크**
> - **모든 Docker 시스템 데이터**
> 
> ⚠️ **이 작업은 되돌릴 수 없으며 다른 Docker 애플리케이션에 영향을 줍니다!**
> 
> Docker 환경을 완전히 초기화하고 싶을 때만 사용하세요.

### 방법 1: 개별 명령어 실행
```bash
$ docker rmi $(docker images -q) -f
$ docker system prune -a
$ docker volume prune
$ docker network prune
$ sudo rm -rf ./conf/docker/data
```

### 방법 2: 자동화 스크립트 사용
더 안전하고 체계적인 정리를 위해 제공되는 스크립트를 사용하세요:

> [!CAUTION]
> **위험: 이 스크립트는 시스템의 모든 Docker 리소스를 삭제합니다!**
> 
> `remove_all.sh` 스크립트는 Cloud-Migrator (cm-mayfly)의 안정적인 동작을 위해 깨끗한 환경을 구축하도록 설계되었습니다. cm-mayfly를 통해 설치되지 않은 **모든 Docker 관련 데이터**를 제거합니다.
> 
> ⚠️ **이 작업은 되돌릴 수 없으며 다른 Docker 애플리케이션에 영향을 줍니다!**

```bash
$ ./remove_all.sh
```