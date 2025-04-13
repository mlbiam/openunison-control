#!/bin/bash

export VERSION="0.0.13"

rm -rf target
mkdir -p target

env GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o ./target/ouctl-$VERSION-macos .
env GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o ./target/ouctl-$VERSION-macos-arm64 . 
env GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ./target/ouctl-$VERSION-linux .
env GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o ./target/ouctl-$VERSION-win.exe .

#upx ./target/ouctl-$VERSION-macos
#upx ./target/ouctl-$VERSION-linux
#upx ./target/ouctl-$VERSION-win.exe

echo "Creating MacOS Intel"
mkdir target/darwin
cp ./target/ouctl-$VERSION-macos target/darwin/ouctl
chmod +x target/darwin/ouctl
cp LICENSE target/darwin/
cd target/darwin/
zip ouctl-$VERSION-macos.zip ./ouctl LICENSE
cd ../../
mv target/darwin/ouctl-$VERSION-macos.zip target/
rm -rf target/darwin

echo "Creating MacOS ARM64"
mkdir target/darwin-arm64
cp ./target/ouctl-$VERSION-macos-arm64 target/darwin-arm64/ouctl
chmod +x target/darwin-arm64/ouctl
cp LICENSE target/darwin-arm64/
cd target/darwin-arm64/
zip ouctl-$VERSION-macos-arm64.zip ./ouctl LICENSE
cd ../../
mv target/darwin-arm64/ouctl-$VERSION-macos-arm64.zip target/
rm -rf target/darwin-arm64


echo "Creating Linux Intel"
mkdir target/linux
cp ./target/ouctl-$VERSION-linux target/linux/ouctl
chmod +x target/linux/ouctl
cp LICENSE target/linux/
cd target/linux/
zip ouctl-$VERSION-linux.zip ./ouctl LICENSE
cd ../../
mv target/linux/ouctl-$VERSION-linux.zip target/
rm -rf target/linux

echo "Creating Windows Intel"
mkdir target/win
cp ./target/ouctl-$VERSION-win.exe target/win/ouctl.exe
cp LICENSE target/win/
cd target/win/
zip ouctl-$VERSION-win.zip ./ouctl.exe ./LICENSE
cd ../../
mv target/win/ouctl-$VERSION-win.zip target/
rm -rf target/win





export MACOS_SHA256=$(sha256sum ./target/ouctl-$VERSION-macos.zip | awk '{print $1}')
export MACOS_ARM64_SHA256=$(sha256sum ./target/ouctl-$VERSION-macos-arm64.zip | awk '{print $1}')
export LINUX_SHA256=$(sha256sum ./target/ouctl-$VERSION-linux.zip | awk '{print $1}')
export WIN_SHA256=$(sha256sum ./target/ouctl-$VERSION-win.zip | awk '{print $1}')

cat ouctl.yaml | sed "s/_VERSION_/$VERSION/g" | sed "s/_MAC_ARM64_SHA_/$MACOS_ARM64_SHA256/g" | sed "s/_MAC_SHA_/$MACOS_SHA256/g" | sed "s/_LINUX_SHA_/$LINUX_SHA256/g" | sed "s/_WIN_SHA_/$WIN_SHA256/g" | sed "s/_REPO_/$1/g" > target/ouctl.yaml

aws s3 sync ./target/ s3://tremolosecurity-maven/repository/$1/



