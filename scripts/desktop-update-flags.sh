#!/bin/sh
set -eu
version=$1
printf '%s' "-X github.com/egoist/mygo.packageVersion=$version -X github.com/egoist/mygo.packageIdentifier=dev.local.mygo-player"
if [ -n "${MYGO_PLAYER_UPDATE_PUBLIC_KEY:-}" ]; then
  case "$MYGO_PLAYER_UPDATE_PUBLIC_KEY" in
    *[!A-Za-z0-9/+=]*) printf '\nInvalid desktop update public key.\n' >&2; exit 1 ;;
  esac
  printf '%s' " -X github.com/egoist/mygo.packageUpdateFeed=https://github.com/shi747826/mygo-player -X github.com/egoist/mygo.packageUpdateKey=$MYGO_PLAYER_UPDATE_PUBLIC_KEY"
fi
printf '\n'
