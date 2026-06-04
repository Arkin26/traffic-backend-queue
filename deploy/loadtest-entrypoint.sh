#!/bin/sh
set -e
mkdir -p /loadtest/jobs /loadtest/results
chmod -R 777 /loadtest/jobs /loadtest/results
exec /usr/local/bin/loadtest-runner
