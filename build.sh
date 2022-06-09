#!/bin/bash

export VERSION="0.0.1"

rm -rf target
mkdir -p target

env GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o ./target/ouctl-$VERSION-macos . 
env GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ./target/ouctl-$VERSION-linux .
env GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o ./target/ouctl-$VERSION-win.exe .

upx ./target/ouctl-$VERSION-macos
upx ./target/ouctl-$VERSION-linux
upx ./target/ouctl-$VERSION-win.exe


mkdir target/darwin
cp ./target/ouctl-$VERSION-macos target/darwin/ouctl
chmod +x target/darwin/ouctl
cp LICENSE target/darwin/
cd target/darwin/
zip ouctl-$VERSION-macos.zip ./ouctl LICENSE
cd ../../
mv target/darwin/ouctl-$VERSION-macos.zip target/
rm -rf target/darwin

mkdir target/linux
cp ./target/ouctl-$VERSION-linux target/linux/ouctl
chmod +x target/linux/ouctl
cp LICENSE target/linux/
cd target/linux/
zip ouctl-$VERSION-linux.zip ./ouctl LICENSE
cd ../../
mv target/linux/ouctl-$VERSION-linux.zip target/
rm -rf target/linux

mkdir target/win
cp ./target/ouctl-$VERSION-win.exe target/win/ouctl.exe
cp LICENSE target/win/
cd target/win/
zip ouctl-$VERSION-win.zip ./ouctl.exe ./LICENSE
cd ../../
mv target/win/ouctl-$VERSION-win.zip target/
rm -rf target/win





export MACOS_SHA256=$(shasum -a 256 ./target/ouctl-$VERSION-macos.zip | awk '{print $1}')
export LINUX_SHA256=$(shasum -a 256 ./target/ouctl-$VERSION-linux.zip | awk '{print $1}')
export WIN_SHA256=$(shasum -a 256 ./target/ouctl-$VERSION-win.zip | awk '{print $1}')

cat ouctl.yaml | sed "s/_VERSION_/$VERSION/g" | sed "s/_MAC_SHA_/$MACOS_SHA256/g" | sed "s/_LINUX_SHA_/$LINUX_SHA256/g" | sed "s/_WIN_SHA_/$WIN_SHA256/g" > target/ouctl.yaml

#aws s3 sync ./target/ s3://tremolosecurity-maven/repository/$1/



