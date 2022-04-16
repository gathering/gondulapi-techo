# Tech:Online Backend

Main repo: [Tech:Online](https://github.com/gathering/tech-online)

Forked from the [Gondul API](https://github.com/gathering/gondulapi) repo for Tech:Online 2020. The 2020 version by Kristian Lyngstøl can be found in the [Tech:Online](https://github.com/gathering/tech-online) repo history. The 2021 and 2022 versions can be found here.

## Description

See [Gondul API](https://github.com/gathering/gondulapi) for more details on the underlying framework.

## Development

### Setup and Run App with Docker Compose

You don't have to use Docker or Docker Compose, but it makes it easier.

1. (First time) Create local config: `cp dev/config.json dev/config.local.json`
1. (First time) Start the DB (detatched): `docker-compose -f dev/docker-compose.yml up -d db`
1. (First time) Apply schema to DB: `dev/db-prepare.sh`
1. Build and start everything: `docker-compose -f dev/docker-compose.yml up --build [-d]`
1. Seed example data: `dev/seed.sh`
1. Profit.

### Development Miscellanea

- Check linting errors: `golint ./...`

## Miscellanea

- This does not feature any kind of automatic DB migration, so you need to manually migrate when upgrading with an existing database (re-applying the schema file for new tables and manually editing existing tables).

## TODO

### General

- Add docs comment to all packages, with consistent formatting.
- Bump Go and dependency versions.
- Implement OpenID Connect or OAuth 2.0.
- Cleanup admin-by-path stuff and associated "ForAdmin" stuff where admin stuff was on separate endpoints.
- From "database_string" to actual parameters.
- Order results by some attribute for certain endpoints.
- Add periodic cleanup of expired tokens.
- Normalize UUIDs from path/query params before comparing in database to avoid missing a match due to case sensitivity for something insensitive.
- Create permanent access tokens through API.
- Remove temporary, custom endpoints (`/custom/track-stations/` and `/custom/station-tasks-tests/`).
- Make usae of `_id` in DB and JSON fields more consistend. And `-id` in query params.

### Desirable Changes from 2021

- UUIDs are nice but not so nice to remember for manual API calls. Maybe find a way to support both UUIDs and composite keys (e.g. track ID + station shortname) in a clean way?
- Make sure the `print-*.sh` scripts aren't needed.
- Better authn/authz! OAuth2 and app tokens and stuff. No more separate "admin" endpoints. See the OAuth something branch. Not implemented this year because of limited time and considerable frontend changes required.
- Split station state "active" into "ready" and "in-use" or something and move timeslot binding to timeslot.
- Get rid of the temporary "custom" endpoints.
- The DB-layer Select() is nice for dynamic "where"s but makes joins, sorting, limiting etc. kinda impossible. Maybe split out the build-where part and allow using it in manual SQL queries?
- Key-value set of variables for each station (e.g. IP addresses for use in docs templating).
- Endpoints with PATCH semantics, e.g. for easily changing station attributes like state and credentials. Requires changes to gondulapi in order to support the PATCH method and for the DB layer to support both PUT and PATCH semantics.
