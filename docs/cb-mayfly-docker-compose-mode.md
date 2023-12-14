
## `cm-mayfly`의 `Docker Compose 모드`를 이용한 Cloud-Migrator 설치 및 실행 가이드

이 가이드에서는 `cm-mayfly`의 `Docker Compose 모드`를 이용하여 Cloud-Migrator 시스템을 구축 및 실행하는 방법에 대해 소개합니다. 


## 순서
1. 개발환경 준비
1. 필요사항 설치
   1. Golang
   1. Docker
   1. Docker Compose
1. cm-mayfly 소스코드 다운로드
1. 환경설정 확인 및 변경
1. cm-mayfly 소스코드 빌드
1. cm-mayfly 이용하여 Cloud-Migrator 실행
1. Cloud-Migrator 실행상태 확인
1. [참고] 프레임워크별 컨테이너 구성 및 API Endpoint


## 개발환경 준비

[권장사항]
- Ubuntu 20.04
- Golang 1.15 또는 그 이상

## 필요사항 설치

### Golang 설치
- https://golang.org/doc/install 에서 설명하는 방법대로 설치합니다.

<details>
  <summary>[클릭하여 예시 보기]</summary>
  
```bash
# Golang 다운로드
wget https://go.dev/dl/go1.21.4.linux-amd64.tar.gz

# 기존 Golang 삭제 및 압축파일 해제
rm -rf /usr/local/go && tar -C /usr/local -xzf go1.21.4.linux-amd64.tar.gz

# ~/.bashrc 또는 ~/.zshrc 등에 다음 라인을 추가
export PATH=$PATH:/usr/local/go/bin

# 셸을 재시작하고 다음을 실행하여 Go 버전 확인
go version
```
</details>

### Docker 설치
- https://docs.docker.com/engine/install/ubuntu/ 에서 설명하는 방법대로 설치합니다.

<details>
  <summary>[클릭하여 예시 보기]</summary>
  
```bash
# 기존에 Docker 가 설치되어 있었다면 삭제
sudo apt remove docker docker-engine docker.io containerd runc

# Docker 설치를 위한 APT repo 추가
sudo apt update

sudo apt install \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg \
    lsb-release

curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg

# x86_64 / amd64
echo \
  "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo apt update

sudo apt install docker-ce docker-ce-cli containerd.io
```
</details>

### Docker Compose 설치
- APT 패키지 매니저를 이용하여 설치합니다.
```bash
sudo apt install docker-compose
```

## cm-mayfly 소스코드 다운로드
```bash
git clone https://github.com/cm-mayfly/cm-mayfly.git
```

## 도커 서비스 정의
Cloud-Migrator 시스템 구성에 필요한 서비스 정보를 `cm-mayfly/docker-compose-mode-files/docker-compose.yaml` 파일에 정의합니다. (현재는 PoC단계라 아직 Cloud-Migrator 시스템의 정식 Docker 이미지가 없기에 유사한 Cloud-Barista의 cb-spider와 cb-tumblebug 도커 이미지를 예시로 제공합니다.)

## cm-mayfly 소스코드 빌드
```bash
cd cm-mayfly/src
go build -o mayfly main.go
```

## cm-mayfly 이용하여 Cloud-Migrator 실행
```bash
./mayfly

# 모드를 고르는 단계가 나오면, 1: Docker Compose 모드 선택

./mayfly run
```

## Cloud-Migrator 실행상태 확인
```bash
./mayfly info
```


## Cloud-Migrator 중지
```bash
./mayfly stop
```

## [참고] 프레임워크별 컨테이너 구성 및 API Endpoint
| Framework별 Container Name | REST-API Endpoint |
|---|---|
| cb-spider | http://{{host}}:1024/spider |
| --- |
| cb-tumblebug | http://{{host}}:1323/tumblebug |
| --- |

