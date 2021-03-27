#!/bin/bash

# Seed some example data into the backend.
# Requires the backend to be running.

ENDPOINT="localhost:8080/api/"

# Users
echo
echo "Seeding users ..."
ENDPOINT_USER="${ENDPOINT}user/"
curl -sSf -X POST $ENDPOINT_USER --data '{"id": "396345b4-553a-4254-97dc-778bea02a86a", "user_name": "hon", "email_address": "hon@example.net", "first_name": "Håvard", "last_name": "Nordstrand"}'
curl -sSf -X POST $ENDPOINT_USER --data '{"user_name": "hon2", "email_address": "hon@example.com", "first_name": "Håvard2", "last_name": "Nordstrand2"}'

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
curl -sSf -X POST $ENDPOINT_DOCUMENT --data '{"id": "396345b4-553a-4254-97dc-878bea02a86b", "family": "demo", "shortname": "demo", "name": "Demo!", "content": "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "content_format": "plaintext"}'
curl -sSf -X POST $ENDPOINT_DOCUMENT --data '{"family": "reference", "shortname": "part2", "sequence": 2, "name": "Title for part 2", "content": "This is *markup* more or less. This is `code`.", "content_format": "markdown"}'
curl -sSf -X POST $ENDPOINT_DOCUMENT --data '{"family": "reference", "shortname": "part3", "sequence": 3, "content": "Nameless."}'

# Tracks
echo
echo "Seeding tracks ..."
ENDPOINT_TRACK="${ENDPOINT}track/"
curl -sSf -X POST $ENDPOINT_TRACK --data '{"id": "net1", "type": "net", "station_permanent": true}'
curl -sSf -X POST $ENDPOINT_TRACK --data '{"id": "server1", "type": "server", "station_permanent": false, "station_create_url": "https://example.net/create", "station_destroy_url": "https://example.net/destroy", "station_count_max": 10}'

# Tasks
echo
echo "Seeding tasks ..."
ENDPOINT_TRACK="${ENDPOINT}task/"
curl -sSf -X POST $ENDPOINT_TRACK --data '{"track": "net1", "shortname": "task1", "name": "Do the first thing", "description": "Desc desc desc", "sequence": 1}'

# Stations
echo
echo "Seeding stations ..."
ENDPOINT_STATION="${ENDPOINT}station/"
curl -sSf -X POST $ENDPOINT_STATION --data '{"id": "1932481b-4126-4cf3-8913-49d0faff75f4", "track": "net1", "shortname": "station1", "status": "active", "credentials": "ssh 10.10.10.10 -p 1000\npassword abclol", "notes": "Idk, broken or smtn.\n\nAAAA"}'

# Timeslots
echo
echo "Seeding timeslots ..."
ENDPOINT_TIMESLOT="${ENDPOINT}timeslot/"
curl -sSf -X POST $ENDPOINT_TIMESLOT --data '{"user": "396345b4-553a-4254-97dc-778bea02a86a", "track": "server1"}'
curl -sSf -X POST $ENDPOINT_TIMESLOT --data '{"user": "396345b4-553a-4254-97dc-778bea02a86a", "track": "net1", "station_shortname": "station1", "begin_time": "2021-03-27T12:12:18.927291Z", "end_time": "2021-03-27T13:12:18.927291Z"}'

# Tests
echo
echo "Seeding tests (won't conflict) ..."
ENDPOINT_TESTS="${ENDPOINT}tests/"
curl -sSf -X POST $ENDPOINT_TESTS --data \
'[
    {"track": "net1", "task_shortname": "task1", "shortname": "test1", "station_shortname": "station1", "name": "Testerino 1", "description": "Testus testicus", "sequence": 1, "status_success": true},
    {"track": "net1", "task_shortname": "task1", "shortname": "test2", "station_shortname": "station1", "name": "Testerino 2", "description": "Testus testicus 2", "sequence": 2, "status_success": false, "status_description": "Failed to ping the thing."}
]'
