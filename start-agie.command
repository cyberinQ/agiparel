#!/bin/bash
cd "$(dirname "$0")/compose"
echo "Starting Agiparel OS Docker Containers..."
docker compose --file docker-compose.dev.yml up --build -d
cd ..
echo "Launching Agie Interactive Onboarding Assistant..."
go run scripts/agie_interview.go
