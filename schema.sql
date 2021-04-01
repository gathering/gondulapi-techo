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

-- Users table
CREATE TABLE public.users (
    token text NOT NULL UNIQUE,
    username text NOT NULL UNIQUE,
    display_name text NOT NULL,
    email_address text NOT NULL
);
CREATE UNIQUE INDEX public_users_token_index ON public.users (token);
CREATE UNIQUE INDEX public_users_username_index ON public.users (username);

-- Document families table
CREATE TABLE public.document_families (
    id text NOT NULL UNIQUE,
    name text NOT NULL
);
CREATE UNIQUE INDEX public_document_families_id_index ON public.document_families (id);

-- Documents table
CREATE TABLE public.documents (
    family text NOT NULL,
    shortname text NOT NULL,
    sequence integer,
    name text NOT NULL,
    content text NOT NULL,
    content_format text NOT NULL,
    last_change timestamp with time zone NOT NULL,
    UNIQUE (family, shortname)
);
CREATE UNIQUE INDEX public_documents_family_shortname_index ON public.documents (family, shortname);

-- Tracks table
CREATE TABLE public.tracks (
    id text NOT NULL UNIQUE,
    type text NOT NULL,
    name text
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
    timeslot text NOT NULL,
    UNIQUE (track, shortname)
);
CREATE UNIQUE INDEX public_stations_id_index ON public.stations (id);

-- Timeslots table
CREATE TABLE public.timeslots (
    id text NOT NULL UNIQUE,
    user_token text NOT NULL,
    track text NOT NULL,
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
    timeslot text,
    name text NOT NULL,
    description text NOT NULL,
    sequence int,
    timestamp timestamp with time zone NOT NULL,
    status_success boolean NOT NULL,
    status_description text NOT NULL,
    UNIQUE (track, task_shortname, shortname, station_shortname, timeslot)
);
CREATE UNIQUE INDEX public_tests_id_index ON public.tests (id);
