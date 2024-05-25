#!/bin/bash

export VERSION="0.0.13"

rm -rf target
mkdir -p target


cat ouctl.yaml | sed "s/_VERSION_/$VERSION/g" | sed "s/_MAC_SHA_/$MACOS_SHA256/g" | sed "s/_LINUX_SHA_/$LINUX_SHA256/g" | sed "s/_WIN_SHA_/$WIN_SHA256/g" > target/ouctl.yaml

aws s3 sync ./target/ s3://tremolosecurity-maven/repository/$1/



