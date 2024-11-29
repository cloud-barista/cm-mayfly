#!/bin/bash

# 모든 Docker 이미지 삭제
echo "All Docker images deleting..."
if [ -n "$(docker images -q)" ]; then
  echo "docker rmi \$(docker images -q) -f"
  docker rmi $(docker images -q) -f
else
  echo "The docker image to delete does not exist."
  echo "All docker images have already been deleted."
fi

echo
# 모든 컨테이너 중지 및 삭제
echo "Stopping and deleting all Docker containers..."
if [ -n "$(docker ps -aq)" ]; then
  echo "docker stop \$(docker ps -aq)"
  docker stop $(docker ps -aq)
  echo "docker rm \$(docker ps -aq)"
  docker rm $(docker ps -aq)
else
  echo "No containers to stop or delete."
fi

echo
# 모든 사용되지 않는 Docker 시스템 리소스 정리
echo "docker system prune -a -f"
docker system prune -a -f

echo
# 모든 볼륨 삭제
echo "Deleting all Docker volumes..."
if [ -n "$(docker volume ls -q)" ]; then
  echo "docker volume rm \$(docker volume ls -q)"
  docker volume rm $(docker volume ls -q)
else
  echo "No volumes to delete."
fi

echo
# 모든 네트워크 삭제
echo "docker network prune -f"
docker network prune -f

echo
# Docker 시스템 리소스 정보 표시
echo "docker system df"
docker system df

echo
echo "All Docker resources have been deleted."
echo "To delete the data directory of the subsystems, run the following command."
echo "==========================="
echo "sudo rm -rf ./data"
echo "==========================="
echo ""