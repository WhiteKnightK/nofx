#!/bin/bash

# This script runs the backend correctly by targeting only main.go
# to avoid conflicts with other utility files in the root directory.

echo "🚀 Starting NOFX Backend..."
go run main.go
