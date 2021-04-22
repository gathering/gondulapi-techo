# Gondul API (Tech:Online 2021 Edition)

A temporary gondulapi fork for Tech:Online 2021.

## Description

This is the API engine that will be used for the Gondul backend in the
future. At present, this is very much a work in progress and should NOT be
used unless you talk to me directly about it first - unless you like
breaking things.

The design goals are:

1. Make it very hard to do the wrong thing.
2. Enforce/ensure HTTP RESTful best behavior.
3. Minimize boilerplate-code
4. Make prototyping fun and easy.

To achieve this, we aim that users of the Gondul API will work mainly with
organizing their own data types and how they are interconnected, and not
worry about how that is parsed to/from JSON or checked for type-safety.

The HTTP-bit is pretty small, but important. It's worth noting that users
of the api-code will *not* have access to any information about the caller.
This is a design decision - it's not your job, it's the job of the
HTTP-server (which is represented by the Gondul API engine here). If your
data types rely on data from the client, you're doing it wrong.

The database-bit can be split into a few categories as well. But mainly, it
is an attempt to make it unnecessary to write a lot of boiler-plate to get
sensible behavior. It is currently written with several known flaws, or
trade-offs, so it's not suited for large deployments, but can be considered
a POC or research.

In general, the DB engine uses introspection to figure out how to figure
out how to retrieve and save an object. The Update mechanism will only
update fields that are actually provided (if that is possible to detect!).

## Development

### Setup with Docker Compose

1. (First time) Create local config: `cp dev/config.json dev/config.local.json`
1. (First time) Start the DB (detatched): `docker-compose -f dev/docker-compose.yml up -d db`
1. (First time) Apply schema to DB: `dev/prepare-db.sh`
1. Build and start everything: `docker-compose -f dev/docker-compose.yml up --build [-d]`
1. Seed example data: `dev/seed.sh`
1. Profit.

### Notes

- Check linting errors: `golint ./...`

## Miscellanea

- This does not feature any kind of automatic DB migration, so you need to manually migrate when upgrading with an existing database (re-applying the schema file for new tables and manually editing existing tables).

## Desirable Changes

(Written after the event.)

- UUIDs are nice but not so nice to remember for manual API calls. Maybe find a way to support both UUIDs and composite keys (e.g. track ID + station shortname) in a clean way?
- Make sure the `print-*.sh` scripts aren't needed.
- Better authn/authz! OAuth2 and app tokens and stuff. No more separate "admin" endpoints. See the OAuth something branch. Not implemented this year because of limited time and considerable frontend changes required.
- Split station state "active" into "ready" and "in-use" or something and move timeslot binding to timeslot.
- Get rid of the temporary "custom" endpoints.
- The DB-layer Select() is nice for dynamic "where"s but makes joins, sorting, limiting etc. kinda impossible. Maybe split out the build-where part and allow using it in manual SQL queries?
- Key-value set of variables for each station (e.g. IP addresses for use in docs templating).
- Endpoints with PATCH semantics, e.g. for easily changing station attributes like state and credentials. Requires changes to gondulapi in order to support the PATCH method and for the DB layer to support both PUT and PATCH semantics.
