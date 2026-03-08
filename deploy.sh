#!/bin/bash

PID=$(ps aux | grep './spo' | grep -v grep | awk '{print $2}')

if [ -n "$PID" ]; then
    echo "Killing process $PID"
    kill $PID
    sleep 1
fi

echo "Building..."
go build -o spo

echo "Starting..."
nohup ./spo > output.log 2>&1 &

echo "Done. PID: $!"
