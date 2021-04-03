#!/bin/bash

# Queries and prints stuff from the DB.

set -eu

manage/db-cmd.sh <<<"select t.track, u.username, u.display_name, t.notes, t.id as timeslot from timeslots as t join users as u on t.user_token = u.token left join stations as s on t.id = s.timeslot where (t.end_time is null or t.end_time > now()) and s.timeslot is null order by t.track asc;"
