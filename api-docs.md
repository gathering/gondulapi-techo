# API Docs

## General

- All endpoints support `?pretty` to pretty print the JSON.
- All listing endpoints support `?limit=<n>` to limit the number of returned objects.
- Some listing endpoints support `?brief` to hide less important fields, to make the dataset smaller when they're not needed.
- Put may have patch semantics.

## Users

**Warning**: Will probably change when AuthN/Z is implemented.

- `/users/[?user_name=<>]`: Get users.
- `/user/[id]`: Get/post/put/delete a user.

## Documents

- `/document-families/`: Get address families.
- `/document-family/`: Get/post/put/delete an document family.
- `/documents/[?family=<>][&shortname=<>]`: Get documents.
- `/document/[id]`: Get/post/put/delete a document.

## Tracks

- `/tracks/[?type=<>]`: Get tracks.
- `/track/[id]`: Get/post/put/delete a track.
- `/track/<id>/new-station`: Post to manually allocate a dynamically allocated station (server track).

## Stations

- `/stations/[?track=<>][&shortname=<>][&status=<>]`: Get stations.
- `/station/[id]`: Get/post/put/delete a station.
- `/station/<id>/destroy`: Post to manually destroy a dynamically allocated station (server track).

## Timeslots

- `/timeslots/[?user=<>][&track=<>][&station_shortname=<>]`: Get timeslots.
- `/timeslot/<id>`: Get/post/put/delete a timeslot.

## Tasks

- `/tasks/[?track=<>][&shortname=<>]`: Get tasks.
- `/task/[id]`: Get/post/put/delete a task.

## Tests

- `/tests/[?track=<>][&task_shortname=<>][&shortname=<>][&station_shortname=<>][&timeslot=<>][&latest]`: Get/post tests.
- `/test/[id]`: Get/post a test.
