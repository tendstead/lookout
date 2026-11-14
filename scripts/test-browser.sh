#!/bin/sh
set -eu
project="lookout-e2e-$(date +%s)-$$"
compose() { docker compose -p "$project" -f compose.verify.yaml "$@"; }
cleanup() { compose down -v --remove-orphans >/dev/null; }
trap cleanup EXIT INT TERM
compose up -d --wait db app
compose run --rm browser
compose stop db
compose run --rm browser node --input-type=module -e 'for (const [path,code] of [["/healthz",200],["/readyz",503]]) { const r=await fetch("http://lookout.test:8080"+path); if(r.status!==code) throw new Error(path+": "+r.status); } console.log("Database-outage lifecycle check passed")'
compose start db
# Test graceful termination after recovery without touching the user's database.
compose stop -t 12 app
id=$(compose ps -aq app)
code=$(docker inspect "$id" --format '{{.State.ExitCode}}')
[ "$code" = 0 ] || { echo "Ungraceful shutdown: $code"; exit 1; }
# Unreachable database must reject startup within the configured deadline.
compose run --rm --no-deps -e DATABASE_URL=postgres://unused@127.0.0.1:1/unused app >/dev/null 2>&1 && { echo "Unexpected startup success"; exit 1; }
echo "Startup failure and graceful shutdown checks passed"
