#!/bin/sh
set -eu
project="lookout-test-$(date +%s)-$$"
cleanup() { docker compose -p "$project" -f compose.test.yaml down -v --remove-orphans >/dev/null; }
trap cleanup EXIT INT TERM
docker compose -p "$project" -f compose.test.yaml up --abort-on-container-exit --exit-code-from test test
