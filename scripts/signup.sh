#!/bin/sh
set -eu

curl --fail-with-body --request POST "${SERVICE_URL:-http://localhost:8080}/signup" \
  --header 'Content-Type: application/json' \
  --data '{"creator_id":"creator_42","email":"creator@example.com","asset_id":"asset_9","title":"Night Shift"}'
