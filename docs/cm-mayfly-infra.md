
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
  - [mc-datamanager 인증 정보 설정](#mc-datamanager-인증-정보-설정)
- [Cloud-Migrator 인프라 구축](#cloud-migrator-인프라-구축)
- [Cloud-Migrator 실행상태 확인](#cloud-migrator-실행상태-확인)
- [Cloud-Migrator 업데이트](#cloud-migrator-업데이트)
  - [기본 업데이트](#기본-업데이트)
  - [버전 체크 및 업데이트 확인](#버전-체크-및-업데이트-확인)
- [Cloud-Migrator 중지](#cloud-migrator-중지)
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
$ git clone https://github.com/cm-mayfly/cm-mayfly.git
```

## 소스코드 빌드
최상위 폴더에 빌드된 실행 파일이 함께 배포되며, 만약 소스 코드 빌드가 필요한 경우에는 README에 설명된 go 설치 방법과 make 명령의 빌드 방법을 참고합니다.
```bash
$ cd cm-mayfly
$ make
```

## 환경설정 확인 및 변경
Cloud-Migrator 시스템 구성에 필요한 정보는 `./conf/docker` 폴더 하위에 정의되어 있으니 시스템 구성전에 `./conf/docker/docker-compose.yaml` 파일 및 `./conf/docker/conf` 폴더의 내용들을 살펴 보고 필요한 경우 수정합니다.


### mc-datamanager 인증 정보 설정
mc-data-manger 서브 시스템은 `CSP를 이용하기 위한 인증 정보가 필요`합니다.
현재는 profile.json 파일을 이용한 설정 방식만 제공하므로 mc-datamanager를 이용하고 싶으면 인프라 구축전에 반드시 `./conf/docker/conf/mc-data-manger/data/var/run/data-manager/profile/profile.json` 파일에 `CSP별 인증 정보를 등록`하세요.

필요한 경우, 인프라 구축 후에 위 `profile.json` 파일의 내용을 수정해도 됩니다.


## Cloud-Migrator 인프라 구축
아래 명령을 실행하면 도커 기반 인프라가 자동으로 구축되며 실행 과정이 화면에 출력됩니다.
```bash
$ ./mayfly infra run
```

만약, Cloud-Migrator 시스템을 구축하려는 시스템 환경이 Clean한 환경이 아니라서 `./mayfly infra run` 명령만으로는 제대로 실행되지 않는 설치 문제가 발생할 경우에는 [Docker 전체 환경 정리](#docker-전체-환경-정리) 섹션의 내용을 확인해서 시스템 환경을 먼저 깔끔하게 정리 후 실행하는 것을 추천드립니다.   


설치 과정을 보고 싶지 않다면 -d 옵션이나 --detach 플래그를 사용해서 설치 과정을 백그라운드로 실행할 수 있습니다.
```bash
$ ./mayfly infra run -d
```


특정 프레임워크만 실행하고 싶으면 아래처럼 실행합니다.   
예를 들어, cb-tumbleug을 실행하고 싶은 경우..
```bash
$ ./mayfly infra run cb-tumblebug
```

## Cloud-Migrator 실행상태 확인
설치된 인프라 및 서브 프레임워크들의 상태를 확인할 수 있습니다.

### 기본 사용법
```bash
$ ./mayfly infra info
```

### 옵션 설명
- `-a, --all`: 모든 컨테이너 상태 표시 (실행 중인 컨테이너뿐만 아니라 중지된 컨테이너도 포함)
  - **주의**: 완전히 삭제된 컨테이너는 표시되지 않습니다

### 사용 예시
```bash
# 실행 중인 컨테이너만 표시 (기본값)
$ ./mayfly infra info

# 모든 컨테이너 상태 표시 (중지된 컨테이너 포함)
$ ./mayfly infra info -a
$ ./mayfly infra info --all
```

실행 결과 예시
```
[Get info for Cloud-Migrator runtimes]


[v]Status of Cloud-Migrator runtimes
NAME             IMAGE                                  COMMAND                   SERVICE          CREATED      STATUS                PORTS
airflow-mysql    mysql:8.0-debian                       "docker-entrypoint.s…"    airflow-mysql    4 days ago   Up 4 days             0.0.0.0:3306->3306/tcp, :::3306->3306/tcp, 33060/tcp
airflow-redis    redis:7.2-alpine                       "docker-entrypoint.s…"    airflow-redis    8 days ago   Up 4 days (healthy)   0.0.0.0:6379->6379/tcp, :::6379->6379/tcp
airflow-server   cloudbaristaorg/airflow-server:edge    "/bin/bash -c '\n    …"   airflow-server   4 days ago   Up 4 days             0.0.0.0:5555->5555/tcp, :::5555->5555/tcp, 0.0.0.0:8080->8080/tcp, :::8080->8080/tcp
ant-postgres     timescale/timescaledb:latest-pg16      "docker-entrypoint.s…"    ant-postgres     8 days ago   Up 4 days (healthy)   0.0.0.0:5432->5432/tcp, :::5432->5432/tcp
cb-mapui         cloudbaristaorg/cb-mapui:0.9.3         "npm start"               cb-mapui         8 days ago   Up 4 days (healthy)   0.0.0.0:1324->1324/tcp, :::1324->1324/tcp
cb-spider        cloudbaristaorg/cb-spider:edge         "/root/go/src/github…"    cb-spider        8 days ago   Up 4 days (healthy)   0.0.0.0:1024->1024/tcp, 0.0.0.0:2048->2048/tcp
cb-tumblebug     cloudbaristaorg/cb-tumblebug:edge      "/app/src/cb-tumbleb…"    cb-tumblebug     8 days ago   Up 4 days (healthy)   0.0.0.0:1323->1323/tcp
cm-ant           cloudbaristaorg/cm-ant:edge            "./ant"                   cm-ant           8 days ago   Up 4 days (healthy)   0.0.0.0:8880->8880/tcp, :::8880->8880/tcp
cm-beetle        cloudbaristaorg/cm-beetle:edge         "/app/cm-beetle"          cm-beetle        5 days ago   Up 4 days (healthy)   0.0.0.0:8056->8056/tcp, :::8056->8056/tcp
cm-butterfly     cloudbaristaorg/cm-butterfly:edge      "./docker_entrypoint…"    cm-butterfly     5 days ago   Up 4 days             0.0.0.0:1234->1234/tcp, :::1234->1234/tcp
cm-cicada        cloudbaristaorg/cm-cicada:edge         "/cm-cicada"              cm-cicada        4 days ago   Up 4 days (healthy)   0.0.0.0:8083->8083/tcp, :::8083->8083/tcp
cm-grasshopper   cloudbaristaorg/cm-grasshopper:edge    "/cm-grasshopper"         cm-grasshopper   8 days ago   Up 4 days (healthy)   0.0.0.0:8084->8084/tcp, :::8084->8084/tcp
cm-honeybee      cloudbaristaorg/cm-honeybee:edge       "/cm-honeybee"            cm-honeybee      8 days ago   Up 4 days (healthy)   0.0.0.0:8081->8081/tcp, :::8081->8081/tcp
cm-mayfly        dev4unet/cm-mayfly:v0.2.0              "bash"                    cm-mayfly        8 days ago   Up 4 days
etcd             gcr.io/etcd-development/etcd:v3.5.14   "/usr/local/bin/etcd…"    etcd             8 days ago   Up 4 days (healthy)   0.0.0.0:2379-2380->2379-2380/tcp, :::2379-2380->2379-2380/tcp

[v]Status of Cloud-Migrator runtime images
CONTAINER           REPOSITORY                       TAG                 IMAGE ID            SIZE
airflow-mysql       mysql                            8.0-debian          ccb4819cef05        611MB
airflow-redis       redis                            7.2-alpine          97ed3031282d        40.7MB
airflow-server      cloudbaristaorg/airflow-server   edge                e80252a32ec3        1.46GB
ant-postgres        timescale/timescaledb            latest-pg16         2bbb52e38008        699MB
cb-mapui            cloudbaristaorg/cb-mapui         0.9.3               308de57eadc9        513MB
cb-spider           cloudbaristaorg/cb-spider        edge                b241e15bba26        386MB
cb-tumblebug        cloudbaristaorg/cb-tumblebug     edge                101876d9e57f        117MB
cm-ant              cloudbaristaorg/cm-ant           edge                9691839034bf        178MB
cm-beetle           cloudbaristaorg/cm-beetle        edge                6601bd684734        114MB
cm-butterfly        cloudbaristaorg/cm-butterfly     edge                22ddc7154d44        41.9MB
cm-cicada           cloudbaristaorg/cm-cicada        edge                afe8229dab34        44.8MB
cm-grasshopper      cloudbaristaorg/cm-grasshopper   edge                965cac894be3        450MB
cm-honeybee         cloudbaristaorg/cm-honeybee      edge                8986d1772357        54.9MB
cm-mayfly           dev4unet/cm-mayfly               v0.2.0              7b3e509bf7d6        146MB
etcd                gcr.io/etcd-development/etcd     v3.5.14             13b135926ee2        57.9MB
```


## Cloud-Migrator 업데이트
Cloud-Migrator 서브 시스템들의 최신 버전으로 업데이트하고 싶은 경우 update 명령으로 현재 환경을 최신 버전으로 재구축할 수 있습니다.

### 기본 업데이트
```bash
$ ./mayfly infra update
```

### 버전 체크 및 업데이트 확인
`mayfly infra update` 명령은 업데이트 전에 각 서비스의 버전 상태를 체크하고 사용자에게 확인을 요청합니다.

#### 동작 방식
1. **로컬 이미지 버전 확인**: 로컬에 다운로드된 이미지 버전
2. **docker-compose.yaml 버전 확인**: 설정 파일에 정의된 버전
3. **Docker Hub 태그 업데이트 확인**: docker-compose.yaml에 정의된 특정 태그의 최신 상태
4. **버전 비교 및 표시**: 각 서비스별 버전 상태를 테이블로 표시
5. **사용자 확인**: 업데이트가 필요한 경우 사용자에게 확인 요청

#### 출력 예시
```
🔍 Checking version updates...

📊 Version Comparison:
┌─────────────────────┬───────────────┬───────────────┬───────────────┐
│ Service             │ Local         │ Compose       │ Latest        │
├─────────────────────┼───────────────┼───────────────┼───────────────┤
│ cm-ant              │ 0.3.8         │ 0.4.0         │ 0.4.0         │ ●
│ cb-tumblebug        │ 0.4.0         │ 0.4.0         │ -             │ ✓
│ cb-spider           │ not_installed │ 0.11.5        │ 0.11.5        │ ✗
└─────────────────────┴───────────────┴───────────────┴───────────────┘

Legend:
✓ All versions match
● Local version differs from docker-compose.yaml (update needed)
✗ Image not installed locally

Do you want to proceed with the update? (y/N): y
```

#### 사용법
```bash
# 전체 서비스 업데이트 (버전 체크 포함)
$ ./mayfly infra update

# 특정 서비스만 업데이트 (버전 체크 포함)
$ ./mayfly infra update -s cm-ant
$ ./mayfly infra update -s cb-tumblebug
```

#### 특징
- **docker-compose.yaml 기준**: 설정 파일에 정의된 버전으로 로컬 환경 맞춤
- **스마트한 버전 관리**: 로컬 버전과 설정 파일 버전 비교
- **사용자 친화적**: 업데이트 전 명확한 정보 제공
- **안전한 업데이트**: 사용자 확인 후에만 업데이트 진행
- **호환성 보장**: docker-compose.yaml에 정의된 버전 우선 사용



## Cloud-Migrator 중지
일부 또는 전체 프레임워크를 잠시 중지할 때 사용합니다.
```bash
$ ./mayfly infra stop
```


특정 프레임워크만 중지하고 싶으면 아래처럼 실행합니다.   
예를 들어, cb-tumbleug을 중지하고 싶은 경우..
```bash
$ ./mayfly infra stop cb-tumbleug
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
| `-s, --service` | 특정 서비스만 대상으로 지정 | `-s cb-tumblebug` |
| `-t, --tail` | 마지막 N줄부터 출력 (0은 처음부터 모든 로그) | `--tail 50` |
| `--since` | 특정 시간 이후의 로그만 출력 | `--since 1h` |
| `--follow` | 실시간으로 로그를 따라가기 (기본값: true) | `--follow` |
| `--no-follow` | follow 모드 비활성화 (로그 확인 후 종료) | `--no-follow` |


### 주요 서비스 이름
- `cb-tumblebug`: CB-Tumblebug 서비스
- `cm-ant`: CM-Ant 서비스  
- `cm-butterfly`: CM-Butterfly 서비스
- `cm-cicada`: CM-Cicada 서비스
- `cm-grasshopper`: CM-Grasshopper 서비스
- `cm-honeybee`: CM-Honeybee 서비스
- `cm-beetle`: CM-Beetle 서비스
- `cb-spider`: CB-Spider 서비스


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
$ ./mayfly infra logs --tail 0

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

# cm-butterfly 서비스의 마지막 100줄 + 2시간 전부터
$ ./mayfly infra logs -s cm-butterfly --tail 100 --since 2h
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

아래 명령으로 실행된 모든 컨테이너가 종료 및 삭제되며 생성된 네트워크도 삭제됩니다.
```bash
$ ./mayfly infra remove
```

사용된 이미지도 함께 삭제합니다.
```bash
$ ./mayfly infra remove --images
또는
$ ./mayfly infra remove -i
```

생성된 볼륨된 함께 삭제합니다.   
(주의) 볼륨이 삭제되면 저장된 데이터도 모두 삭제됩니다.
```bash
$ ./mayfly infra remove --volumes
또는
$ ./mayfly infra remove -v
```

컨테이너를 비롯하여 네트워크 및 이미지와 볼륨 등 모든 자원을 삭제합니다.   
완전히 최초 상태로 재구축하고 싶거나 더 이상 필요 없을 때 사용하세요.
```bash
./mayfly infra remove --images --volumes
또는 
./mayfly infra remove -i -v
또는
./mayfly infra remove --all
```

**참고**: `./conf/docker/data` 폴더 하위에 각 서브 프레임워크(컨테이너) 이름의 폴더가 생성되며 Data나 Log를 비롯하여 보관이 필요한 경우에 사용됩니다.   
`--volumes` 옵션은 Docker 볼륨만 삭제하며, 볼륨 마운트에 의해 로컬에 생성된 `./conf/docker/data` 폴더는 삭제되지 않습니다.   
만약, 컨테이너에 의해 로컬에 저장된 데이터까지 완전히 삭제화하고 싶다면 `./conf/docker/data` 폴더 하위의 모든 폴더들을 수동으로 삭제해야 합니다.



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
$ cd conf/docker
$ ./remove_all.sh
```