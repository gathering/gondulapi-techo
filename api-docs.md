# API Docs

## General

- All endpoints support `?pretty` to pretty print the JSON.
- All listing endpoints support `?limit=<n>` to limit the number of returned objects (WIP).
- Some listing endpoints support `?brief` to hide less important fields, to make the dataset smaller when they're not needed (WIP).
- PUT may have PATCH semantics.

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
| `/documents/[?family=<>][&shortname=<>]` | `GET` | Get documents. | Public. |
| `/document/[id]` | `GET`, `POST`, `PUT`, `DELETE` | Get/post/put/delete a document. | Public (read) and admin. |

### Tracks

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/tracks/[?type=<>]` | `GET` | Get tracks. | Public. |
| `/track/[id]` | `GET`, `POST`, `PUT`, `DELETE` | Get/post/put/delete a track. | Public (read) and admin. |

### Stations

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/stations/[?track=<>][&shortname=<>][&status=<>][&user-token=<>]` | `GET` | Get stations. | Public (read without credentials). |
| `/station/[id]` | `GET`, `POST`, `PUT`, `DELETE` | Get/post/put/delete a station. To allocate or destroy the backing station (server track using VMs), use the special endpoints for that instead. | Assigned participant (read), public (read without credentials) and admin. |
| `/track/<track-id>/provision-station` | `POST` | Provision a station for a track (server track). | Admin. |
| `/station/<id>/terminate` | `POST` | Terminate a station (server track). The station will be markled as terminated (not deleted). | Admin. |
| `/admin/stations/[?track=<>][&shortname=<>][&status=<>]` | `GET` | Get stations with credentials. | Public (read without credentials) and admin. |

### Timeslots

Timeslots are the participation objects for a user and a track. The start time, end time and station gets filled in later.

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/timeslots/?user-token=<>[&track=<>]` | `GET` | Get timeslots for a user. | Public (secret user token). |
| `/timeslot/[id][?user-token=<>]` | `GET`, `POST`, `PUT`, `DELETE` | Get/post/put/delete a timeslot for a user. | Public (secret user token). |
| `/admin/timeslots/[?user-token=<>][&track=<>][&station-shortname=<>][&state={unassigned\|active}]` | `GET` | Get timeslots. | Admin. |

### Tasks

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/tasks/[?track=<>][&shortname=<>]` | `GET` | Get tasks. | Public. |
| `/task/[id]` | `GET`, `POST`, `PUT`, `DELETE` | Get/post/put/delete a task. | Public (read) and admin. |

### Tests

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/tests/[?track=<>][&task-shortname=<>][&shortname=<>][&station-shortname=<>][&timeslot=<>][&latest]` | `GET`, `POST`, `DELETE` | Get/post/delete tests. | Public (read) and admin. |
| `/test/[id]` | `GET`, `POST`, `DELETE` | Get/post/delete a test. | Public (read) and admin. |

## Examples

### Provision And Terminate Dynamic Server Stations

**Provision**:

```
$ curl -D - http://localhost:8080/api/track/server/provision-station --data ''
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8
Etag: 2cc55706087bee87cdfc23b307a16b6d8ff92936cab78d62ccc73a1781118114
Location: /api/station/303485ed-9d10-4558-a402-1f345ce12855
Date: Tue, 30 Mar 2021 19:09:33 GMT
Content-Length: 33

{"Affected":1,"Ok":1,"Failed":0}
```

Follow or parse the `Location` header for info about the created resource.

**Terminate**:

```
$ curl -D - http://localhost:8080/api/station/303485ed-9d10-4558-a402-1f345ce12855/terminate --data ''
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Etag: 2cc55706087bee87cdfc23b307a16b6d8ff92936cab78d62ccc73a1781118114
Date: Tue, 30 Mar 2021 19:09:56 GMT
Content-Length: 33

{"Affected":1,"Ok":1,"Failed":0}

```
