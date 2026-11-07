#!/bin/sh
# Init container (postgres image). Loads the WSO2 schemas into empty databases.
# Each database is probed for one of its tables, so restarts are no-ops.
set -eu
export PGPASSWORD="$DB_PASSWORD"
host=postgres

until pg_isready -q -h "$host" -U "$DB_USERNAME"; do echo "waiting for postgres"; sleep 2; done

init() {
  db=$1 probe=$2
  shift 2
  present=$(psql -h "$host" -U "$DB_USERNAME" -d "$db" -tAc "SELECT to_regclass('$probe') IS NOT NULL")
  if [ "$present" = "t" ]; then
    echo "$db: schema present"
    return
  fi
  for script in "$@"; do
    echo "$db: loading $script"
    psql -q -v ON_ERROR_STOP=1 --single-transaction -h "$host" -U "$DB_USERNAME" -d "$db" -f "$script"
  done
}

init WSO2_IDENTITY_DB idn_base_table /dbscripts/identity/postgresql.sql /dbscripts/consent/postgresql.sql
init WSO2_SHARED_DB reg_cluster_lock /dbscripts/postgresql.sql
init WSO2_AGENT_IDENTITY_DB um_role /dbscripts/identity/agent/postgresql.sql
# Secondary JDBC user store for acceptance tests (user management tables of the shared DB).
init WSO2_TEST_USERSTORE_DB um_user /dbscripts/postgresql.sql
