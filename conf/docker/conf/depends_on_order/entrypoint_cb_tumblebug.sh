#!/bin/sh
# Docker 이미지의 Entrypoint를 Overwrite하게 됨.

# entrypoint.sh
echo "Start checking the Ready state of containers that depend on cb-tumblebug..."

# Wait for cm-spider to be ready
until /app/tool/mayfly rest get http://cb-spider:1024/spider/readyz; do
  echo "Waiting for cm-spider to be readyz..."
  sleep 2
done
echo "cm-spider is readyz..."


# Execute the original CMD or any passed arguments
exec "$@"
# cb-tumblebug은 cmd로 실행되는 것이 아니라, entrypoint로 실행되어야 함.
# 또는 exec "$@"을 정의하고 docker-compose.yaml에서 command: ["/app/src/cb-tumblebug"]로 값을 넣어주어야 함.
#/app/src/cb-tumblebug
