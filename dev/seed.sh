#!/bin/bash

# Seed some example data into the backend.
# Requires the backend to be running.

set -eu

ENDPOINT="localhost:8080/api/"
ENDPOINT_DOC_FAMILY="localhost:8080/api/doc/family/"
ENDPOINT_RESULTS="localhost:8080/api/test/"

# Docs
curl -sSf -X POST $ENDPOINT_DOC_FAMILY --data '{"family": "reference", "shortname": "demo", "name": "Demo!", "content": "https://www.youtube.com/watch?v=dQw4w9WgXcQ"}'
curl -sSf -X POST $ENDPOINT_DOC_FAMILY --data '{"family": "reference", "shortname": "part2", "name": "Title for part 2", "content": "This is *markup* more or less. This is `code`."}'
curl -sSf -X POST $ENDPOINT_DOC_FAMILY --data '{"family": "reference", "shortname": "part3", "content": "Nameless."}'

# Results
curl -sSf -X POST $ENDPOINT_RESULTS --data '{"track": "net", "station": 2, "title": "Random tittel.", "description": "Random desc.", "status": "ok", "hash": "knis"}'

echo
echo "Done!"
