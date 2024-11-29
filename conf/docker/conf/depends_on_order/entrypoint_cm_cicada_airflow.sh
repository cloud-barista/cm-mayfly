#!/bin/sh

# entrypoint.sh
echo "Start checking the Ready state of containers that depend on airflow-server..."

# Wait for airflow-mysql to be ready
# /app/tool/mayfly tool mysqlping -v
until /app/tool/mayfly tool mysqlping; do
    echo 'Waiting for MySQL(airflow-mysql) to be ready...'
    sleep 2
done

# Execute the original CMD or any passed arguments
# echo "==============================="
# echo "$@"
# echo "==============================="
exec "$@"
