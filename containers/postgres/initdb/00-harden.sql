-- Runs once, on first init of an empty data directory (the image's /docker-entrypoint-initdb.d).
-- Cells are provisioned later, on the live server, by provision-cell.sh.
--
-- A cell role connects to its own database only: no role but the administrator may connect to
-- the maintenance database or the template databases. template0 already refuses connections.
REVOKE CONNECT, TEMPORARY ON DATABASE postgres FROM PUBLIC;
REVOKE CONNECT, TEMPORARY ON DATABASE template1 FROM PUBLIC;
