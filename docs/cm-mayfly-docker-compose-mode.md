
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
```bash
$ ./mayfly infra info
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
```bash
$ ./mayfly infra update
```



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

## Docker 전체 환경 정리
Cloud-Migrator 인프라를 구축하기 전에 사용했던 도커 환경과의 충돌로 인해 모든 환경을 초기화하고 싶은 경우 아래 명령어로 초기화가 가능합니다.

[주의] 아래 명령을 실행하면 docker로 생성된 이미지 / 볼륨 / 네트워크 등의 모든 정보기 삭제됩니다.
```bash
$ docker rmi $(docker images -q) -f
$ docker system prune -a
$ docker volume prune
$ docker network prune
$ sudo rm -rf ./conf/docker/data
```
