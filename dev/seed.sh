#!/bin/bash

# Seed some example data into the backend.
# Requires the backend to be running.

set -u

ENDPOINT="localhost:8080/api/"

# Users
echo
echo "Seeding users (no conflict) ..."
ENDPOINT_USER="${ENDPOINT}user/"
curl -sSf ${ENDPOINT_USER}396345b4-553a-4254-97dc-778bea02a86a -X PUT --data '{"id": "396345b4-553a-4254-97dc-778bea02a86a", "username": "hon", "email_address": "hon@example.net", "display_name": "Håvard1"}'
curl -sSf ${ENDPOINT_USER}396345b4-553a-4254-97dc-778bea02a86b -X PUT --data '{"id": "396345b4-553a-4254-97dc-778bea02a86b", "username": "hon2", "email_address": "hon@example.com", "display_name": "Håvard2"}'
curl -sSf ${ENDPOINT_USER}396345b4-553a-4254-97dc-778bea02a86b -X PUT --data '{"id": "396345b4-553a-4254-97dc-778bea02a86b", "username": "hon2", "email_address": "hon@example.com", "display_name": "Håvard5"}'

# Document families
echo
echo "Seeding document families ..."
ENDPOINT_DOCUMENT_FAMILY="${ENDPOINT}document-family/"
curl -sSf $ENDPOINT_DOCUMENT_FAMILY --data '{"id": "demo", "name": "Demo!"}'
curl -sSf $ENDPOINT_DOCUMENT_FAMILY --data '{"id": "reference", "name": "Reference"}'

# Documents
echo
echo "Seeding documents ..."
ENDPOINT_DOCUMENT="${ENDPOINT}document/"
ENDPOINT_DOCUMENTS="${ENDPOINT}documents/"
curl -sSf $ENDPOINT_DOCUMENT --data '{"family": "demo", "shortname": "demo", "name": "Demo!", "content": "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "content_format": "plaintext"}'
curl -sSf $ENDPOINT_DOCUMENT --data '{"family": "reference", "shortname": "part2", "sequence": 2, "name": "Title for part 2", "content": "This is *markup* more or less. This is `code`.", "content_format": "markdown"}'
curl -sSf $ENDPOINT_DOCUMENT --data '{"family": "reference", "shortname": "part3", "sequence": 3, "content": "Nameless."}'
curl -sSf ${ENDPOINT_DOCUMENT}reference/part5/ -X PUT --data '{"family": "reference", "shortname": "part5", "sequence": 5, "content": "Nameless again."}'
curl -sSf ${ENDPOINT_DOCUMENT}reference/part5/ -X PUT --data '{"family": "reference", "shortname": "part5", "sequence": 5, "content": "Named!."}'
curl -sSf ${ENDPOINT_DOCUMENTS} -X PUT --data \
'[
    {"family": "reference", "shortname": "part10", "sequence": 10, "content": "a"},
    {"family": "reference", "shortname": "part11", "sequence": 11, "content": "b"}
]'

# Tracks
echo
echo "Seeding tracks ..."
ENDPOINT_TRACK="${ENDPOINT}track/"
curl -sSf $ENDPOINT_TRACK --data '{"id": "net", "type": "net", "name": "Network"}'
curl -sSf $ENDPOINT_TRACK --data '{"id": "server", "type": "server", "name": "Server"}'

# Tasks
echo
echo "Seeding tasks ..."
ENDPOINT_TRACK="${ENDPOINT}task/"
curl -sSf $ENDPOINT_TRACK --data '{"track": "net", "shortname": "task1", "name": "Do the first thing", "description": "Desc desc desc", "sequence": 1}'

# Timeslots
echo
echo "Seeding timeslots ..."
ENDPOINT_TIMESLOT="${ENDPOINT}admin/timeslot/"
curl -sSf $ENDPOINT_TIMESLOT --data '{"user_id": "396345b4-553a-4254-97dc-778bea02a86a", "track": "server"}'
curl -sSf $ENDPOINT_TIMESLOT --data '{"id": "86fb0380-647f-471f-9df0-d61ff38f6e98", "user_id": "396345b4-553a-4254-97dc-778bea02a86a", "track": "net", "begin_time": "2020-03-27T12:12:18.927291Z", "end_time": "3020-03-27T13:12:18.927291Z"}'

# Stations
echo
echo "Seeding stations ..."
ENDPOINT_STATION="${ENDPOINT}station/"
curl -sSf $ENDPOINT_STATION --data '{"id": "1932481b-4126-4cf3-8913-49d0faff75f4", "track": "net", "shortname": "1", "name": "Station #1", "status": "active", "credentials": "ssh 10.10.10.10 -p 1000\npassword abclol", "notes": "Idk, broken or smtn.\n\nAAAA"}'
curl -sSf $ENDPOINT_STATION --data '{"id": "1932481b-4126-4cf3-8913-49d0faff75f5", "track": "net", "shortname": "2", "name": "Station #2", "status": "maintenance"}'
curl -sSf ${ENDPOINT_STATION}1932481b-4126-4cf3-8913-49d0faff75f5 -X PUT --data '{"id": "1932481b-4126-4cf3-8913-49d0faff75f5", "track": "net", "shortname": "2", "name": "Station #2", "status": "active", "timeslot": "86fb0380-647f-471f-9df0-d61ff38f6e98"}'

# Tests
echo
echo "Seeding tests (won't conflict) ..."
ENDPOINT_TESTS="${ENDPOINT}tests/"
curl -sSf $ENDPOINT_TESTS --data \
'[
    {"track": "net", "task_shortname": "task1", "shortname": "test1", "station_shortname": "1", "name": "Testerino 1", "description": "Testus testicus", "sequence": 1, "status_success": true},
    {"track": "net", "task_shortname": "task1", "shortname": "test2", "station_shortname": "1", "name": "Testerino 2", "description": "Testus testicus 2", "sequence": 2, "status_success": false, "status_description": "Failed to ping the thing."},
    {"track": "net", "task_shortname": "task1", "shortname": "test1", "station_shortname": "2", "name": "Testerino 1 on station 2", "description": "Testus testicus", "sequence": 1, "status_success": true}
]'
