# Gondul API (Tech:Online Edition)

A temporary gondulapi fork for Tech:Online.

Note that it still uses the module name `github.com/gathering/gondulapi` even though this repo is called `gondulapi-techo`.

(Forgive me for _temporarily_ converting the README to Md.)

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

## Authentication & Authorization

- The authn/authz is handled by Varnish, in front if the backend.
- Frontend users are authenticated (to themselved) using OAuth2 against IdP XXX.
- Backend endpoints not prefixed with `/admin/` allow GET/HEAD without authn.
- POST/PUT/DELETE to any endpoint and any method to endpoints prefixed `/admin/` _generally_ requires basic auth.
- Frontend users are allowed to GET/POST/PUT/DELETE their own timeslot which is indexed by a _very secret_ token known only to the users' authenticated clients.

## Miscellanea

- This does not feature any kind of automatic DB migration, so you need to manually migrate when upgrading with an existing database (re-applying the schema file for new tables and manually editing existing tables).
