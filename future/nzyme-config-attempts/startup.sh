#!/bin/bash
set -e

echo "Starting nzyme-node initialization..."

# Replace environment variables in config template
envsubst < /etc/nzyme/nzyme.conf.tmp > /etc/nzyme/nzyme.conf

echo "Configuration file created at /etc/nzyme/nzyme.conf"

# Start nzyme
echo "Starting nzyme..."
exec java ${JAVA_OPTS} -jar -Dlog4j.configurationFile=file:///etc/nzyme/log4j2-debian.xml /usr/share/nzyme/nzyme.jar -c /etc/nzyme/nzyme.conf