#!/bin/bash

echo "=========================================="
echo "  Docker Environment Cleanup Script"
echo "=========================================="
echo ""
echo "To build a clean environment for error-free reinstallation,"
echo "all Docker environments and existing data volumes installed on the system will be deleted."
echo ""
echo "[Note]"
echo "If the script stops running for a long time even after multiple re-executions,"
echo "please reboot the system."
echo "=========================================="
echo ""
echo "Do you want to proceed with the cleanup? (y/N): "
read -r proceed
if [[ ! "$proceed" =~ ^[Yy]$ ]]; then
    echo "Cleanup cancelled."
    exit 0
fi
echo ""

# 1. 모든 컨테이너 강제 중지 및 삭제 (먼저 실행)
echo "Force stopping and deleting all Docker containers..."
if [ -n "$(docker ps -aq)" ]; then
  echo "Force stopping all containers..."
  docker stop $(docker ps -aq) 2>/dev/null || true
  
  # 10초 대기 후 강제 종료
  echo "Waiting 10 seconds for graceful shutdown..."
  sleep 10
  
  # 아직 실행 중인 컨테이너 강제 종료
  if [ -n "$(docker ps -q)" ]; then
    echo "Force killing remaining containers..."
    docker kill $(docker ps -q) 2>/dev/null || true
  fi
  
  # 모든 컨테이너 삭제 (실행 중인 것도 포함)
  echo "Removing all containers..."
  docker rm -f $(docker ps -aq) 2>/dev/null || true
else
  echo "No containers to stop or delete."
fi

echo
# 2. 모든 네트워크 삭제 (컨테이너 삭제 후)
echo "Deleting all Docker networks..."
if [ -n "$(docker network ls -q --filter type=custom)" ]; then
  echo "docker network rm \$(docker network ls -q --filter type=custom)"
  docker network rm $(docker network ls -q --filter type=custom) 2>/dev/null || true
else
  echo "No custom networks to delete."
fi

echo
# 3. 모든 볼륨 삭제
echo "Deleting all Docker volumes..."
if [ -n "$(docker volume ls -q)" ]; then
  echo "docker volume rm \$(docker volume ls -q)"
  docker volume rm $(docker volume ls -q)
else
  echo "No volumes to delete."
fi

echo
# 4. 모든 이미지 삭제
echo "Deleting all Docker images..."
if [ -n "$(docker images -q)" ]; then
  echo "docker rmi \$(docker images -q) -f"
  docker rmi $(docker images -q) -f
else
  echo "No images to delete."
fi

echo
# 5. 시스템 정리 (마지막에 실행)
echo "Performing final system cleanup..."
docker system prune -a -f

echo
# Docker 시스템 리소스 정보 표시
echo "docker system df"
docker system df

echo
echo "All Docker resources have been deleted."
echo ""
echo "The data directory contains local volume data created by the subsystems."
echo "Do you want to delete the data directory as well? (y/N): "
read -r response
if [[ "$response" =~ ^[Yy]$ ]]; then
    echo "Deleting data directory..."
    sudo rm -rf ./data
    echo "Data directory has been deleted."
else
    echo "Data directory deletion skipped."
    echo "To delete the data directory manually, run the following command:"
    echo "sudo rm -rf ./data"
fi
echo ""