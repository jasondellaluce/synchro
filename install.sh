#!/bin/sh

LATEST=$(curl -sI https://github.com/jasondellaluce/synchro/releases/latest | awk '/location: /{gsub("\r","",$2);split($2,v,"/");print substr(v[8],2)}')

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT
cd "$TMPDIR"

curl --fail -LS "https://github.com/jasondellaluce/synchro/releases/download/v${LATEST}/synchro_${LATEST}_linux_amd64.tar.gz" | tar -xz
if [ -f "synchro" ]; then
    sudo install -o root -g root -m 0755 synchro /usr/local/bin/synchro
    echo "Successfully installed synchro to /usr/local/bin/synchro"
else
    echo "Error: Failed to download or extract synchro."
    exit 1
fi
