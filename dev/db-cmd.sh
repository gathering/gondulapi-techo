#!/bin/bash

# Opens a non-interactive shell to the DBMS, which may be used in scripts etc.
# Requires the database to be running.

set -eu

docker-compose -f dev/docker-compose.yml exec -T db sh -c "psql -U techo techo"
