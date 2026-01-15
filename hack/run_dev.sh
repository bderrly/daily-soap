#!/bin/bash

if pgrep -f "wgo run .* ./cmd/server" > /dev/null; then
    echo "Server is already running."
    exit 0
fi

echo "Starting server..."
CGO_ENABLED=1 wgo run -file .css -file .html -file .gotmpl -file .js ./cmd/server
