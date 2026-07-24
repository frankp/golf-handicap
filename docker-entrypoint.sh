#!/bin/sh
set -eu

if [ "$(id -u)" = "0" ]; then
    exec su-exec "${PUID:-99}:${PGID:-100}" "$@"
fi

exec "$@"
