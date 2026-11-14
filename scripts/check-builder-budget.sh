#!/bin/sh
set -eu
container=buildx_buildkit_lookout-builder0
actual=$(docker inspect "$container" --format '{{.HostConfig.Memory}}|{{.HostConfig.MemorySwap}}|{{.HostConfig.CpuPeriod}}|{{.HostConfig.CpuQuota}}')
[ "$actual" = '2147483648|2147483648|100000|200000' ] || { echo "Lookout builder must enforce 2 GiB RAM, no extra swap, 2 CPUs"; exit 1; }
echo 'BuildKit effective budget verified: 2 GiB RAM, no extra swap, 2 CPUs'
