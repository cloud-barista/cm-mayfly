#!/bin/bash
#
# remove_all.sh — full Docker environment reset for cm-mayfly.
#
# Lives at the cm-mayfly root (next to the `mayfly` binary and Makefile) so it
# can drive `mayfly infra remove` directly. It is self-locating: it cd's to its
# own directory first, so it works no matter where it is invoked from.
#
# Order of operations:
#   1) mayfly-aware cleanup — `mayfly infra remove --clean-all -y` removes the
#      mayfly stack, named volumes, host bind data AND clears the .env
#      VAULT_TOKEN. Without this a stale VAULT_TOKEN survives the wipe and the
#      next `infra run` skips OpenBao init and deadlocks.
#   2) nuclear cleanup — remove ALL remaining Docker resources on the host,
#      including those NOT installed through cm-mayfly.

# Resolve the script's own directory (the mayfly root) and work from there.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR" || exit 1

echo "==========================================="
echo " DANGER: Docker Environment Cleanup Script "
echo "==========================================="
echo ""
echo "🚨 WARNING: This script will DELETE ALL Docker resources on your system!"
echo ""
echo "This script is designed to create a clean environment for stable operation"
echo "of Cloud-Migrator (cm-mayfly). It will remove ALL Docker-related data that"
echo "was NOT installed through cm-mayfly, including:"
echo ""
echo "  • ALL Docker containers (running and stopped)"
echo "  • ALL Docker images"
echo "  • ALL Docker volumes"
echo "  • ALL custom Docker networks"
echo "  • ALL Docker system data"
echo ""
echo "🚨 This action is IRREVERSIBLE and will affect other Docker applications!"
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

# 0. mayfly-aware cleanup first — clears .env VAULT_TOKEN + mayfly volumes/host data.
# Graceful: if the mayfly binary or compose file is missing, or the command
# fails, warn and continue with the nuclear cleanup below.
echo "Running mayfly-aware cleanup (infra remove --clean-all)..."
MAYFLY_BIN=""
if [ -x "$SCRIPT_DIR/mayfly" ]; then
  MAYFLY_BIN="$SCRIPT_DIR/mayfly"
elif [ -x "$SCRIPT_DIR/conf/docker/tool/mayfly" ]; then
  MAYFLY_BIN="$SCRIPT_DIR/conf/docker/tool/mayfly"
fi
if [ -n "$MAYFLY_BIN" ] && [ -f "$SCRIPT_DIR/conf/docker/docker-compose.yaml" ]; then
  "$MAYFLY_BIN" infra remove --clean-all -y \
    || echo "warn: 'mayfly infra remove --clean-all' failed; continuing with nuclear cleanup."
else
  echo "warn: mayfly binary or docker-compose.yaml not found — skipping mayfly-aware cleanup."
fi

echo
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
    sudo rm -rf ./conf/docker/data
    echo "Data directory has been deleted."
else
    echo "Data directory deletion skipped."
    echo "To delete the data directory manually, run the following command:"
    echo "sudo rm -rf ./conf/docker/data"
fi
echo ""
