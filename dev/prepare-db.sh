#!/bin/bash

# Apply the SQL schema to the database.
# Requires the database to be running.

set -eu

# This is pretty much idempotent, so running it in an existing DB is fine.
docker-compose -f dev/docker-compose.yml exec db sh -c "psql -U gondulapi gondulapi < /tmp/schema.sql"
