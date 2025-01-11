#!/bin/bash

echo "Building application..."
go build -o sem1-proj .

echo "Running program..."
nohup ./sem1-proj > output.log