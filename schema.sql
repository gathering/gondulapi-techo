SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;
SET default_tablespace = '';
SET default_with_oids = false;

-- Function to update timestamp (field "time")
CREATE FUNCTION public.upd_timestamp() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.time = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;
ALTER FUNCTION public.upd_timestamp() OWNER TO gondulapi;

-- -- Docs table
-- CREATE TABLE public.docs (
--     family text,
--     shortname text,
--     sequence integer,
--     name text,
--     content text
-- );
-- ALTER TABLE public.docs OWNER TO gondulapi;

-- -- Results table
-- CREATE TABLE public.results (
--     track text,
--     station integer,
--     title text DEFAULT ''::text,
--     description text,
--     status text,
--     task text,
--     participant text,
--     hash text,
--     "time" timestamp with time zone DEFAULT now()
-- );
-- ALTER TABLE public.results OWNER TO gondulapi;
-- CREATE TRIGGER t_name BEFORE UPDATE ON public.results FOR EACH ROW EXECUTE PROCEDURE public.upd_timestamp();

-- -- Participants table
-- CREATE TABLE public.participants (
--     id text,
--     first_name text,
--     last_name text,
--     display_name text,
--     email_address text
-- );
-- ALTER TABLE public.participants OWNER TO gondulapi;
-- CREATE UNIQUE INDEX puniq ON public.participants USING btree (uuid);

-- -- Stations table
-- CREATE TABLE public.stations (
--     stationid integer NOT NULL,
--     jumphost text,
--     net inet,
--     password text NOT NULL,
--     participant text,
--     notes text
-- );
-- ALTER TABLE public.stations OWNER TO gondulapi;

-- -- Status table
-- CREATE TABLE public.status (
--     stationid integer,
--     title text,
--     description text,
--     status text,
--     task text,
--     participantid text
-- );
-- ALTER TABLE public.status OWNER TO gondulapi;

-- -- Tasks table
-- CREATE TABLE public.tasks (
--     sequence integer,
--     short_name text,
--     name text,
--     description text
-- );
-- ALTER TABLE public.tasks OWNER TO gondulapi;

-- -- Timeslots table
-- CREATE TABLE public.timeslots (
--     user text,
--     start timestamp with time zone,
--     end timestamp with time zone,
--     station_id integer
-- );
-- ALTER TABLE public.timeslots OWNER TO gondulapi;

-- Users table
CREATE TABLE public.users (
    id text NOT NULL UNIQUE,
    user_name text NOT NULL UNIQUE,
    email_address text NOT NULL,
    first_name text NOT NULL,
    last_name text NOT NULL
);
CREATE UNIQUE INDEX public_users_id_index ON public.users (id);
CREATE UNIQUE INDEX public_users_user_name_index ON public.users (user_name);

-- Document families table
CREATE TABLE public.document_families (
    id text NOT NULL UNIQUE,
    name text NOT NULL
);
CREATE UNIQUE INDEX public_document_families_id_index ON public.document_families (id);

-- Documents table
CREATE TABLE public.documents (
    id text NOT NULL UNIQUE,
    family text NOT NULL,
    shortname text NOT NULL,
    sequence integer,
    name text NOT NULL,
    content text NOT NULL,
    content_format text NOT NULL,
    last_change timestamp with time zone NOT NULL,
    UNIQUE (family, shortname)
);
CREATE UNIQUE INDEX public_documents_id_index ON public.documents (id);
CREATE UNIQUE INDEX public_documents_family_shortname_index ON public.documents (family, shortname);

-- Tracks table
CREATE TABLE public.tracks (
    id text NOT NULL UNIQUE,
    type text NOT NULL
);
CREATE UNIQUE INDEX public_tracks_id_index ON public.tracks (id);

-- Tasks table
CREATE TABLE public.tasks (
    id text NOT NULL UNIQUE,
    track text NOT NULL,
    shortname text NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    sequence int,
    UNIQUE (track, shortname)
);
CREATE UNIQUE INDEX public_tasks_id_index ON public.tasks (id);

-- Stations table
CREATE TABLE public.stations (
    id text NOT NULL UNIQUE,
    track text NOT NULL,
    shortname text NOT NULL,
    name text NOT NULL,
    status text NOT NULL,
    credentials text NOT NULL,
    notes text NOT NULL,
    UNIQUE (track, shortname)
);
CREATE UNIQUE INDEX public_stations_id_index ON public.stations (id);

-- Timeslots table
CREATE TABLE public.timeslots (
    id text NOT NULL UNIQUE,
    user_id text NOT NULL,
    track text NOT NULL,
    station_shortname text NOT NULL,
    begin_time timestamp with time zone,
    end_time timestamp with time zone
);
CREATE UNIQUE INDEX public_timeslots_id_index ON public.timeslots (id);

-- Tests table
CREATE TABLE public.tests (
    id text NOT NULL UNIQUE,
    track text NOT NULL,
    task_shortname text NOT NULL,
    shortname text NOT NULL,
    station_shortname text NOT NULL,
    timeslot text NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    sequence int,
    timestamp timestamp with time zone NOT NULL,
    status_success boolean NOT NULL,
    status_description text NOT NULL,
    UNIQUE (track, task_shortname, shortname, station_shortname, timeslot)
);
CREATE UNIQUE INDEX public_tests_id_index ON public.tests (id);
