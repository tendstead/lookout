#!/bin/sh
set -eu
builder=lookout-builder
container=buildx_buildkit_lookout-builder0
image=$(docker compose -f compose.builder.yaml config --images)
if ! docker buildx inspect "$builder" >/dev/null 2>&1; then
  docker buildx create --name "$builder" --driver docker-container \
    --driver-opt "image=$image,memory=2g,memory-swap=2g,cpu-period=100000,cpu-quota=200000,restart-policy=no" >/dev/null
fi
cleanup() { docker buildx stop "$builder" >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM
docker buildx inspect --bootstrap "$builder" >/dev/null
actual_image=$(docker inspect "$container" --format '{{.Config.Image}}')
[ "$actual_image" = "$image" ] || { echo "Builder image changed; run docker buildx rm --keep-state lookout-builder and retry"; exit 1; }
sh scripts/check-builder-budget.sh
docker buildx build --builder "$builder" --load --progress plain -t lookout:local .
