#!/bin/bash

# Opens an interactive shell to the DBMS.
# Requires the database to be running.

set -eu

docker-compose -f dev/docker-compose.yml exec db sh -c "psql -U gondulapi gondulapi"
