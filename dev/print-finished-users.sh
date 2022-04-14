#!/bin/bash

# Queries and prints stuff from the DB.

set -eu

manage/db-cmd.sh <<<"select t.track, u.username, u.display_name, t.begin_time, t.end_time from timeslots as t join users as u on t.user_id = u.id where t.end_time < now() order by t.track asc, t.begin_time asc;"
