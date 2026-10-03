#!/bin/sh
set -eu

# This creates a new disposable container; it never opens the production .env.
database=${TEST_DB_NAME:-boops_test_console}
case "$database" in
  boops_test_*) ;;
  *) echo 'TEST_DB_NAME must start with boops_test_' >&2; exit 1 ;;
esac
case "$database" in
  *[!a-zA-Z0-9_]*) echo 'TEST_DB_NAME must contain only letters, digits, and underscores' >&2; exit 1 ;;
esac
port=${TEST_DB_PORT:-33306}
password=${TEST_DB_PASSWORD:-boops-disposable-test}
container="boopsdb-test-$$"
docker run --detach --rm --name "$container" \
  --publish "127.0.0.1:$port:3306" \
  --env "MYSQL_ROOT_PASSWORD=$password" \
  --env "MYSQL_DATABASE=$database" \
  mysql:8.4.10 >/dev/null

ready=false
for attempt in $(seq 1 60); do
  if docker exec "$container" sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -uroot -e "SELECT 1"' >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
if [ "$ready" != true ]; then
  docker stop "$container" >/dev/null
  echo 'Isolated MySQL did not become ready' >&2
  exit 1
fi
printf 'Isolated MySQL is ready. Run tests with these environment variables:\n'
printf 'TEST_DB_HOST=127.0.0.1\nTEST_DB_PORT=%s\nTEST_DB_NAME=%s\nTEST_DB_USER=root\n' "$port" "$database"
printf 'TEST_DB_PASSWORD is the value passed to this script (default: boops-disposable-test).\n'
printf 'Stop and delete the disposable container after testing: docker stop %s\n' "$container"
