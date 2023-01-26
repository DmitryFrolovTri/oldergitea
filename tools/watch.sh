#!/bin/bash
set -euo pipefail

if [[ $1 == '--lite' ]]; then
  export TAGS="bindata sqlite sqlite_unlock_notify"
fi

make watch-frontend &
make watch-backend &

trap 'kill $(jobs -p)' EXIT
wait
