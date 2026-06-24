# Локальный PostgreSQL для phone-orchestrator (Windows)
$ErrorActionPreference = "Stop"
$psql = "C:\Program Files\PostgreSQL\17\bin\psql.exe"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$migration = Join-Path $root "migrations\001_phones.up.sql"
$port = if ($env:ORCHESTRATOR_PG_PORT) { $env:ORCHESTRATOR_PG_PORT } else { "5434" }

if (-not (Test-Path $psql)) {
    Write-Error "PostgreSQL не найден: $psql. Либо docker compose -f deploy/docker-compose.yml up -d postgres"
}

$env:PGPASSWORD = "postgres"
& $psql -h localhost -p $port -U postgres -d postgres -v ON_ERROR_STOP=1 -c @"
DO `$`$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'orchestrator') THEN
    CREATE ROLE orchestrator LOGIN PASSWORD 'orchestrator';
  END IF;
END `$`$;
"@

$dbExists = & $psql -h localhost -p $port -U postgres -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = 'orchestrator'"
if ($dbExists -ne '1') {
    & $psql -h localhost -p $port -U postgres -d postgres -c "CREATE DATABASE orchestrator OWNER orchestrator;"
}

& $psql -h localhost -p $port -U postgres -d orchestrator -f $migration
& $psql -h localhost -p $port -U postgres -d orchestrator -c @"
GRANT ALL ON ALL TABLES IN SCHEMA public TO orchestrator;
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO orchestrator;
GRANT USAGE ON SCHEMA public TO orchestrator;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO orchestrator;
"@

Write-Host "Готово. DSN: postgres://orchestrator:orchestrator@localhost:${port}/orchestrator?sslmode=disable"
