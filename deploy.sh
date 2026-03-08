#!/bin/bash

PID=$(ps aux | grep './spotify-playlist-gen' | grep -v grep | awk '{print $2}')

if [ -n "$PID" ]; then
    echo "Killing process $PID"
    kill $PID
    sleep 1
fi

if [ -f output.log ]; then
    echo "Deleting output.log"
    rm output.log
fi

echo "Building..."
go build -o spotify-playlist-gen

echo "Starting..."
nohup ./spotify-playlist-gen > output.log 2>&1 &

echo "Done. PID: $!"
