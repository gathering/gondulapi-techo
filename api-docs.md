# API Docs

## General

- All endpoints support `?pretty` to pretty print the JSON.
- All listing endpoints support `?limit=<n>` to limit the number of returned objects (WIP).
- Some listing endpoints support `?brief` to hide less important fields, to make the dataset smaller when they're not needed (WIP).
- PUT may have PATCH semantics.
- PUT generally allows creating new resources if they don't already exist.

## Authentication & Authorization

- The authn/authz is handled by Varnish, in front if the backend.
- Frontend users are authenticated (to themselved) using OAuth2 against IdP XXX.
- Backend endpoints not prefixed with `/admin/` allow GET/HEAD without authn.
- POST/PUT/DELETE to any endpoint and any method to endpoints prefixed `/admin/` _generally_ requires basic auth.
- Frontend users are allowed to GET/POST/PUT/DELETE their own timeslot which is indexed by a _very secret_ token known only to the users' authenticated clients.

## Endpoints

### Users

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/user/<token>` | `PUT` | Createor update a user. | Public (write). |
| `/admin/users/[?username=<>][&token=<>]` | `GET` | Get users. | Admin. |

### Documents

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/document-families/` | `GET` | Get address families. | Public. |
| `/document-family/` | `GET`, `POST`, `PUT`, `DELETE` | Get/post/put/delete an document family. | Public (read) and admin. |
| `/documents/[?family=<>][&shortname=<>]` | `GET`, `PUT` | Get og create/update documents. | Public (read) and admin. |
| `/document/[<family-id>/<shortname>]` | `GET`, `POST`, `PUT`, `DELETE` | Get/post/put/delete a document. | Public (read) and admin. |

### Tracks

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/tracks/[?type=<>]` | `GET` | Get tracks. | Public. |
| `/track/[id]` | `GET`, `POST`, `PUT`, `DELETE` | Get/post/put/delete a track. | Public (read) and admin. |
| `/track/<id>/provision-station` | `POST` | Manually provision a station for a the track (server track), which will enter the maintenance state to avoid being assigned. | Admin. |

### Stations

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/stations/[?track=<>][&shortname=<>][&status=<>][&timeslot=<>][&user-token=<>]` | `GET` | Get stations. The credentials will be hidden unless filtering by timeslot ID and providing the correct user token. | Public (read without credentials). |
| `/admin/stations/[?track=<>][&shortname=<>][&status=<>]` | `GET` | Get stations with credentials. | Public (read without credentials) and admin. |
| `/station/[id]` | `GET`, `POST`, `PUT`, `DELETE` | Get/post/put/delete a station. To allocate or destroy the backing station (server track using VMs), use the special endpoints for that instead. | Assigned participant (read), public (read without credentials) and admin. |
| `/admin/station/[id]` | `GET` | Get a station with credentials. | Admin. |
| `/station/<id>/terminate` | `POST` | Manually terminate a station (server track). The station will be markled as terminated (not deleted). | Admin. |

### Timeslots

Timeslots are the participation objects for a user and a track. The start time, end time and station gets filled in later.

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/timeslots/?user-token=<>[&track=<>]` | `GET` | Get timeslots for a user. | Public (secret user token). |
| `/timeslot/[id][?user-token=<>]` | `GET`, `POST` | Get/post a timeslot for a user. With limited access because public. | Public (secret user token). |
| `/admin/timeslots/[?user-token=<>][&track=<>][&station-shortname=<>][&no-time][&not-ended][&assigned-station][&not-assigned-station]` | `GET` | Get timeslots. | Admin. |
| `/admin/timeslot/[id]` | `GET`, `POST`, `PUT`, `DELETE` | Get/post/put/delete a timeslot for a user. | Admin. |
| `/admin/timeslot/<id>/assign-station/` | `POST` | Attempts to find an available station (state ready or provision new) and bind it to the timeslot. May provision new stations (server track). It sets the begin time to now and end time a 1000 years into the future. | Admin. |
| `/admin/timeslot/<id>/finish/` | `POST` | End the timeslot and make the station dirty/terminated. It sets the end time to now. | Admin. |

### Tasks

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/tasks/[?track=<>][&shortname=<>]` | `GET` | Get tasks. | Public. |
| `/task/[id]` | `GET`, `POST`, `PUT`, `DELETE` | Get/post/put/delete a task. | Public (read) and admin. |

### Tests

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/tests/[?track=<>][&task-shortname=<>][&shortname=<>][&station-shortname=<>][&timeslot=<>][&latest]` | `GET`, `POST`, `DELETE` | Get/post/delete tests. If using mass delete, consider making a backup first as a misspelled query arg can nuke the entire table. | Public (read) and admin. |
| `/test/[id]` | `GET`, `POST`, `DELETE` | Get/post/delete a test. | Public (read) and admin. |

### Custom

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/custom/track-stations/<track-id>/` | `GET` | Get track info and all active stations for the specified track ID. | Public. |
| `/custom/station-tasks-tests/<track-id>/<station-shortname>/` | `GET` | Get track info and tasks and tests for the specified track ID and station shortname. | Public. |

## Useful Requests

**Show dirty net track stations**:

`curl "https://techo.gathering.org/api/stations/?track=net&status=dirty&pretty"`

**Show timeslots without times (new registration)**:

`curl -u "<HIDDEN>" "https://techo.gathering.org/api/admin/timeslots/?no-time&pretty"`

**Show timeslots with stations (currently in use)**:

`curl -u "<HIDDEN>" "https://techo.gathering.org/api/admin/timeslots/?assigned-station&pretty"`

**Show timeslots with times and waiting for stations**:

`curl -u "<HIDDEN>" "https://techo.gathering.org/api/admin/timeslots/?not-ended&not-assigned-station&pretty"`

## Examples

### User Registration (User)

Should be called every time the user logs in. It's idempotent. Note that it's possible to just make a fake user by generating a UUID, the frontend auth part is mostly just for convenience.

```
$ curl -D - https://techo.gathering.org/api/user/396345b4-553a-4254-97dc-778bea02a000/ -X PUT --data '{"token":"396345b4-553a-4254-97dc-778bea02a000","username":"hon5550","display_name":"Håvard5550","email_address":"hon5550@example.net"}'

HTTP/1.1 200 OK
...

{"affected":1,"ok":1}
```

### Track Participant Registration (User)

The user can have one active registration (aka timeslot) for each track. This is not idempotent, so check (GET timeslots with user token) if the user already has a registration for the track. The time and station allocation will be handled manually by crew. The participant should ask crew if it wishes to withdraw its registration or make changes to it.

```
$ curl -D - https://techo.gathering.org/api/timeslot/ --data '{"user_token":"396345b4-553a-4254-97dc-778bea02a000","track":"server"}'

HTTP/1.1 201 Created
Location: /api/timeslot/75dff19e-6305-4b9a-883d-1bd6f6b60616/
...

{"affected":1,"ok":1}
```

### Show Own Station With Credentials (User)

Show the station assigned to a timeslot, after the crew has assigned one. Requires both the timeslot ID and user token. Returns a list containing zero or one stations.

```
$ curl -D - https://techo.gathering.org/api/stations/\?timeslot\=75dff19e-6305-4b9a-883d-1bd6f6b60616\&user-token\=396345b4-553a-4254-97dc-778bea02a000\&pretty

HTTP/1.1 200 OK

[
  {
    "id": "1ec64ed3-8cce-48f1-b094-c078cf83480e",
    "track": "server",
    "shortname": "23",
    "name": "vm-knnwpgsi.techo.no",
    "status": "maintenance",
    "credentials": "Username: tech\nPassword: akuOfYL4Aw\nPublic IPv4 address: 185.80.182.120\nPublic IPv6 address: 2a02:d140:c012:41::6\nSSH port: 20005",
    "notes": "FQDN: vm-knnwpgsi.techo.no\nZone: zone-knnwpgsi.techo.no\nVLAN ID: 1054\nVLAN IPv4 Subnet: 10.10.54.2/24",
    "timeslot": "75dff19e-6305-4b9a-883d-1bd6f6b60616"
  }
]
```

### Set Tentative Time for Participant (Admin)

The time is shown to the user and helps the crew organize delegation of stations, but generally has no effect when it comes to internal backend logic (i.e. stations may be assigned with wrong or no time for a timeslot). Finishing a timeslot via the "finish" endpoint will automatically set the end time (and begin time if not set) to now, so setting it e.g. to year 3000 is a useful way to indicate that the timeslot isn't finished yet. And end time that has passed will allow the user to register for a new time slot.

```
$ curl -u "<HIDDEN>" "https://techo.gathering.org/api/admin/timeslot/a6d525e5-34c5-45c1-954b-e8c747779655" -X PUT --data '{
  "id": "a6d525e5-34c5-45c1-954b-e8c747779655",
  "user_token": "396345b4-553a-4254-97dc-778bea02a86a",
  "track": "server",
  "begin_time": "2021-04-01T10:00:00+02:00",
  "end_time": "3000-01-01T00:00:00+02:00"
}'

{"affected":1,"ok":1}
```

### Assign Station to Participant (Admin)

If there is a station available (state "active") or if one can be allocated (server track), this should automatically bind the station to the timeslot. Note that it may take a few seconds to complete if it has to allocate a server station (VM). It sets the begin time to now and end time a 1000 years into the future.

```
$ curl -u "<HIDDEN>" -D - https://techo.gathering.org/api/timeslot/75dff19e-6305-4b9a-883d-1bd6f6b60616/assign-station/ --data ''

HTTP/1.1 303 See Other
Location: /api/station/1ec64ed3-8cce-48f1-b094-c078cf83480e/
...

{"affected":1,"ok":1}
```

### Finish a Time Slot (Admin)

After a user is done with a station, doing this will end the time slot and either terminate the station (server track) or mark it as dirty so the crew may clean it (net track). It sets the end time to now, which allows the user to register a new time slot.

```
curl -u "<HIDDEN>" -D - https://techo.gathering.org/api/timeslot/75dff19e-6305-4b9a-883d-1bd6f6b60616/finish/ --data ''

HTTP/1.1 200 OK
...

{}
```

### Update Station (Admin)

When station details changes, e.g. when a net station timeslot has finished and the station has status "dirty" and needs new credentials (do this AFTER resetting the station). Note that the the GET uses "/admin/" (to get credentials) and PUT doesn't (because the endpoint doesn't exist).

**Get existing station**:

```
$ curl https://techo.gathering.org/api/admin/station/1932481b-4126-4cf3-8913-49d0faff75f5/
{"id":"1932481b-4126-4cf3-8913-49d0faff75f5","track":"net","shortname":"2","name":"Station #2","status":"dirty","credentials":"ssh address, username, whatever, old secret password","notes":"","timeslot":""}

```

**Update existing station**:

```
$ curl https://techo.gathering.org/api/station/1932481b-4126-4cf3-8913-49d0faff75f5/ -X PUT --data '{"id":"1932481b-4126-4cf3-8913-49d0faff75f5","track":"net","shortname":"2","name":"Station #2","status":"active","credentials":"ssh address, username, whatever, NEW secret password","notes":"","timeslot":""}'
{"affected":1,"ok":1}
```

### Manually Provision And Terminate Dynamic Server Stations (Admin)

This is generally only needed for cleanup when something went wrong. The timeslot endpoints also manage dynamic server stations and is recommended to use instead if possible.

**Provision**:

```
$ curl -u "<HIDDEN>" -D - https://techo.gathering.org/api/track/server/provision-station --data ''

HTTP/1.1 201 Created
Location: /api/station/303485ed-9d10-4558-a402-1f345ce12855
...

{"Affected":1,"Ok":1,"Failed":0}
```

Follow or parse the `Location` header for info about the created resource.

**Terminate**:

```
$ curl -u "<HIDDEN>" -D - https://techo.gathering.org/api/station/303485ed-9d10-4558-a402-1f345ce12855/terminate --data ''

HTTP/1.1 200 OK
...

{"Affected":1,"Ok":1,"Failed":0}

```
