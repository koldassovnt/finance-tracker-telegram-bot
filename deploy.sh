#!/bin/sh
# Builds the bot image tagged with the version in VERSION and (re)starts it.
#
# VERSION is the one place the version is written. Bump it for every deploy:
# rebuilding a version that already ran overwrites its image, and with it the
# rollback target, so this refuses to.
#
# Compose reads the tag from VERSION in .env, which this keeps in step. To
# roll back, set VERSION in .env to an older tag still on this host and run
# `docker compose up -d` without --build.
set -eu
cd "$(dirname "$0")"

version=$(tr -d ' \r\n' < VERSION)
if ! printf '%s' "$version" | grep -Eqx '[0-9]+\.[0-9]+\.[0-9]+'; then
	echo "VERSION must look like 1.2.3, got '$version'" >&2
	exit 1
fi
image="family-finance-telegram-bot:$version"
if docker image inspect "$image" >/dev/null 2>&1; then
	echo "$image is already built: bump VERSION for a new deploy" >&2
	exit 1
fi
if [ ! -f .env ]; then
	echo ".env is missing: create it as described in README.md" >&2
	exit 1
fi

if grep -q '^VERSION=' .env; then
	sed -i "s/^VERSION=.*/VERSION=$version/" .env
else
	[ -z "$(tail -c 1 .env)" ] || echo >> .env
	echo "VERSION=$version" >> .env
fi

docker compose up -d --build
