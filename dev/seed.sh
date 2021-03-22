#!/bin/bash

# Seed some example data into the backend.
# Requires the backend to be running.

ENDPOINT="localhost:8080/api/"

# Users
echo
echo "Seeding users ..."
ENDPOINT_USER="${ENDPOINT}user/"
curl -sSf -X POST $ENDPOINT_USER --data '{"id": "396345b4-553a-4254-97dc-778bea02a86a", "user_name": "hon", "email_address": "hon@example.net", "first_name": "Håvard", "last_name": "Nordstrand"}'
curl -sSf -X POST $ENDPOINT_USER --data '{"id": "396345b4-553a-4254-97dc-778bea02a86b", "user_name": "hon2", "email_address": "hon@example.com", "first_name": "Håvard2", "last_name": "Nordstrand2"}'

# Document families
echo
echo "Seeding document families ..."
ENDPOINT_DOCUMENT_FAMILY="${ENDPOINT}document-family/"
curl -sSf -X POST $ENDPOINT_DOCUMENT_FAMILY --data '{"id": "demo", "name": "Demo!"}'
curl -sSf -X POST $ENDPOINT_DOCUMENT_FAMILY --data '{"id": "reference", "name": "Reference"}'

# Documents
echo
echo "Seeding documents ..."
ENDPOINT_DOCUMENT="${ENDPOINT}document/"
curl -sSf -X POST $ENDPOINT_DOCUMENT --data '{"family_id": "demo", "local_id": "demo", "name": "Demo!", "content": "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "content_format": "plaintext"}'
curl -sSf -X POST $ENDPOINT_DOCUMENT --data '{"family_id": "reference", "local_id": "part2", "sequence": 2, "name": "Title for part 2", "content": "This is *markup* more or less. This is `code`.", "content_format": "markdown"}'
curl -sSf -X POST $ENDPOINT_DOCUMENT --data '{"family_id": "reference", "local_id": "part3", "sequence": 3, "content": "Nameless."}'

# Stations
echo
echo "Seeding stations ..."
ENDPOINT_STATION="${ENDPOINT}station/"
curl -sSf -X POST $ENDPOINT_STATION --data '{"id": "1", "status": "active", "endpoint": "10.10.10.10:1000", "password": "hunter2", "notes": "Idk.\n\nAAAA"}'
