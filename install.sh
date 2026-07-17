#!/usr/bin/env bash

# pull from main
git pull origin main

# build project
go build -o dotsync-agent .

# move local to executable path
sudo mv dotsync-agent /usr/local/bin
