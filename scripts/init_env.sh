#!/bin/sh
# Initialize a new checkout without overwriting the developer's existing secrets.
set -eu
cd "$(dirname "$0")/.."
if [ ! -f .env ]; then
    umask 077
    task_jwt_secret=$(openssl rand -hex 32)
    sed '/^[[:space:]]*JWT_SECRET_HEX[[:space:]]*=/d' .env.example > .env
    printf '\nJWT_SECRET_HEX=%s\n' "$task_jwt_secret" >> .env
    echo 'Created local .env with a new JWT secret.'
fi
