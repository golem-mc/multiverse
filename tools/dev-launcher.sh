#!/bin/sh
# Writes dev/launcher.json: the golem version go.mod pins and this repository's plugin packages.
set -eu
cd "$(dirname "$0")/.."
version=$(go list -m -f '{{.Version}}' github.com/golem-mc/golem)
cat > dev/launcher.json <<JSON
{
  "server": "$version",
  "plugins": [
    "..",
    "../inventories",
    "../netherportals",
    "../portals"
  ],
  "updateCheck": false
}
JSON
