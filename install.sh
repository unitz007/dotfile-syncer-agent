#!/usr/bin/env bash

# pull from main
git pull origin main

# build project
go build -o dotfile-agent

# move local to executable path
sudo mv dotfile-agent /usr/local/bin