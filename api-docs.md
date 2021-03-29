# API Docs

## General

- All endpoints support `?pretty` to pretty print the JSON.
- All listing endpoints support `?limit=<n>` to limit the number of returned objects (WIP).
- Some listing endpoints support `?brief` to hide less important fields, to make the dataset smaller when they're not needed (WIP).
- PUT may have PATCH semantics.

## Users

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/admin/users/[?username=<>][&token=<>]` | `GET` | Get users. | Admin. |
| `/user/<token>` | `PUT` | Get/post/put/delete a user. | Public (write). |

## Documents

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/document-families/` | `GET` | Get address families. | Public. |
| `/document-family/` | `GET`, `POST`, `PUT`, `DELETE` | Get/post/put/delete an document family. | Public (read) and admin. |
| `/documents/[?family=<>][&shortname=<>]` | `GET` | Get documents. | Public. |
| `/document/[id]` | `GET`, `POST`, `PUT`, `DELETE` | Get/post/put/delete a document. | Public (read) and admin. |

## Tracks

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/tracks/[?type=<>]` | `GET` | Get tracks. | Public. |
| `/track/[id]` | `GET`, `POST`, `PUT`, `DELETE` | Get/post/put/delete a track. | Public (read) and admin. |
| `/track/<id>/new-station` | `POST` | Manually allocate a station (server track). | Admin. |

## Stations

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/stations/[?track=<>][&shortname=<>][&status=<>]` | `GET` | Get stations. | Public (read without credentials) and admin. |
| `/station/[id]` | `GET`, `POST`, `PUT`, `DELETE` | Get/post/put/delete a station. To allocate or destroy the backing station (server track using VMs), use the special endpoints for that instead. | Assigned participant (read), public (read without credentials) and admin. |
| `/station/<id>/destroy` | `POST` | Manually destroy an allocated station (server track). | Admin. |

## Timeslots

Timeslots are the participation objects for a user and a track. The start time, end time and station gets filled in later.

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/admin/timeslots/[?user-token=<>][&track=<>][&station-shortname=<>]` | `GET` | Get timeslots. | Admin. |
| `/timeslots/?user-token=<>[&track=<>]` | `GET` | Get timeslots for a user. | Public (secret user token). |
| `/timeslot/[id][?user-token=<>]` | `GET`, `POST`, `PUT`, `DELETE` | Get/post/put/delete a timeslot for a user. | Public (secret user token). |

## Tasks

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/tasks/[?track=<>][&shortname=<>]` | `GET` | Get tasks. | Public. |
| `/task/[id]` | `GET`, `POST`, `PUT`, `DELETE` | Get/post/put/delete a task. | Public (read) and admin. |

## Tests

| Endpoint | Methods | Description | Auth |
| - | - | - | - |
| `/tests/[?track=<>][&task-shortname=<>][&shortname=<>][&station-shortname=<>][&timeslot=<>][&latest]` | `GET`, `POST` | Get/post tests. | Public (read) and admin. |
| `/test/[id]` | `GET`, `POST`, `PUT`, `DELETE` | Get/post a test. | Public (read) and admin. |
