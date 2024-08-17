#!/bin/bash

echo "This example will compile the application a few times with"
echo "If the version is 'dev', no update checking will be performed."
echo

rm -rf example-updater
rm -rf  example-server
rm -rf tmp
rm -rf public/*

echo "Building example-server"
go build -o example-server src/example-server/main.go

echo "Running example server"
killall -q example-server
killall -q example-updater
./example-server &

read -n 1 -p "Press any key to start." ignored
echo

go build -ldflags="-X main.version=1.0" -o example-updater src/example-updater/main.go
./example-updater &



for ((minor = 1; minor <= 3; minor++)); do
    sleep 10
    echo "Building example-updater with version set to 1.$minor"
    go build -ldflags="-X main.version=1.$minor" -o example-updater src/example-updater/main.go

    echo "Running ./updater to make update available via example-server"
    echo
    ./updater -o public/ example-updater 1.$minor
done


echo "Shutting down example-server"
killall -q example-server
killall -q example-updater
rm -rf example-updater
rm -rf  example-server
rm -rf tmp
rm -rf public/*