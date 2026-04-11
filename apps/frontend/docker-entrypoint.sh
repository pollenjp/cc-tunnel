#!/bin/sh
set -eu

# Generate /env.js with runtime environment variables
cat > /usr/share/nginx/html/env.js <<EOF
window.__ENV__ = {
  BACKEND_URL: "${BACKEND_URL:-}"
};
EOF

# Delegate to nginx's original entrypoint so the image keeps its standard
# startup behavior, including /docker-entrypoint.d/* hooks and envsubst for
# /etc/nginx/templates/*.template. "$@" forwards Docker CMD as argv unchanged.
exec /docker-entrypoint.sh "$@"
