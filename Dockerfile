# docker build -t cm-mayfly:v0.1.0 .
# docker run -it --rm --name cm-mayfly cm-mayfly:v0.1.0 /bin/bash
# docker exec -it cm-mayfly /bin/bash
# ./mayfly rest get -u default -p default http://cb-spider:1024/spider/readyz

# 베이스 이미지 선택
FROM ubuntu:latest

RUN apt-get update && apt-get install -y iputils-ping curl

# 컨테이너 내의 작업 디렉토리 설정(RUN / CMD / ENTRYPOINT 명령이 실행될 기본 경로)
WORKDIR /app

# 파일 복사 src -> dest
COPY conf /app/conf
COPY bin /app/bin

# 도커 컨테이너에서 실행할 명령어 (이미지 빌드시 실행)
#RUN 설치 명령어
#RUN 

WORKDIR /app/bin
# 컨테이너 실행 시 최초 실행될 default 명령 (1개만 가능)
# 컨테이너 실행 시 명령이 전달되면 CMD는 무시되고 전달 받은 명령이 실행 됨.
CMD ["bash"]

#항상 자동 종료되는 것을 막고 싶으면 위 bash대신 아래 명령어로 대체
#CMD ["tail", "-f", "/dev/null"]

# 항상 실행할 명령 (CMD와 비슷)
# CMD와 달리 컨테이너 실행 시 항상 실행 됨.
# 컨테이너 실행 시 명령어가 전달되면 ENTRYPOINT먼저 실행 후 전달 받은 명령이 실행 됨.
#ENTRYPOINT [ "/app/bin/" ]
