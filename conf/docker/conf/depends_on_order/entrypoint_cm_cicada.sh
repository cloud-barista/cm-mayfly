#!/bin/sh

# entrypoint.sh
echo "Starting cm-cicada..."

# Wait for cm-beetle to be ready
until /app/tool/mayfly rest get http://cm-beetle:8056/beetle/readyz; do
  echo "Waiting for cm-beetle to be readyz..."
  sleep 2
done
echo "cm-beetle is readyz..."

# Wait for cm-grasshopper to be ready
until /app/tool/mayfly rest get http://cm-grasshopper:8084/grasshopper/readyz; do
  echo "Waiting for cm-grasshopper to be readyz..."
  sleep 2
done
echo "cm-grasshopper is readyz..."

# Wait for airflow-server to be ready
# Airflow 3 moved the health endpoint; /health now 404s and points at /api/v2/monitor/health.
until /app/tool/mayfly rest get http://airflow-server:8080/api/v2/monitor/health; do
  echo "Waiting for airflow-server to be health..."
  sleep 2
done
echo "airflow-server is health..."

cp -RpPf /conf $CMCICADA_ROOT/

# Execute the original CMD or any passed arguments
exec "$@"
