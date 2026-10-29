#!/bin/sh
# Author: Sunil Jain, 2026-10-29.
# PostgreSQL official container init: use env-supplied dev login secrets.
set -eu
: "${AGIPAREL_APP_DB_PASSWORD:?missing application secret}"
: "${AGIPAREL_DOC_DB_PASSWORD:?missing worker secret}"
psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -v app_password="$AGIPAREL_APP_DB_PASSWORD" \
  -v doc_password="$AGIPAREL_DOC_DB_PASSWORD" \
  -f /etc/agiparel/bootstrap_roles_v42.sql
