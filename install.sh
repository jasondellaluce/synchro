#!/bin/sh

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
if [ "$OS" != "linux" ]; then
    echo -e "Error: unsupported operating system: $OS.\n\033[1msynchro\033[0m only supports Linux for now."
    exit 1
fi

ARCH=$(uname -m)
case "$ARCH" in
    x86_64)
        ARCH_SUFFIX="amd64"
        ;;
    aarch64)
        ARCH_SUFFIX="arm64"
        ;;
    *)
        echo "Error: Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

LATEST=$(curl -sI https://github.com/jasondellaluce/synchro/releases/latest | awk '/location: /{gsub("\r","",$2);split($2,v,"/");print substr(v[8],2)}')

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT
cd "$TMPDIR"

curl --fail -LS "https://github.com/jasondellaluce/synchro/releases/download/v${LATEST}/synchro_${LATEST}_${OS}_${ARCH_SUFFIX}.tar.gz" | tar -xz
if [ -f "synchro" ]; then
    sudo install -o root -g root -m 0755 synchro /usr/local/bin/synchro
    echo "Successfully installed synchro to /usr/local/bin/synchro"
else
    echo "Error: Failed to download or extract synchro."
    exit 1
fi
