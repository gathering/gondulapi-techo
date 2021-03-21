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

-- Docs table
CREATE TABLE public.docs (
    family text,
    shortname text,
    sequence integer,
    name text,
    content text
);
ALTER TABLE public.docs OWNER TO gondulapi;

-- Results table
CREATE TABLE public.results (
    track text,
    station integer,
    title text DEFAULT ''::text,
    description text,
    status text,
    task text,
    participant text,
    hash text,
    "time" timestamp with time zone DEFAULT now()
);
ALTER TABLE public.results OWNER TO gondulapi;
CREATE TRIGGER t_name BEFORE UPDATE ON public.results FOR EACH ROW EXECUTE PROCEDURE public.upd_timestamp();
