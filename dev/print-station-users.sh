#!/bin/bash

# Queries and prints stuff from the DB.

set -eu

manage/db-cmd.sh <<<"select s.track, s.shortname as station, u.username, u.display_name, t.notes, t.id as timeslot from stations as s join timeslots as t on s.timeslot = t.id join users as u on u.token = t.user_token order by s.track asc, s.shortname asc;"
