#!/bin/bash
# Verifies the two assumptions in
# adr/2026-05/2026-05-06T11:00:45+09:00_01_container_manager_on_vm.md:
#
#   1) A container on Docker's bridge network can reach the GCE metadata
#      server (http://metadata.google.internal/computeMetadata/v1/...).
#   2) The VM Service Account's access token can pull from Artifact Registry.
#
# Exits non-zero on any failure so the calling startup-script can surface the
# result via journald / serial console.
set -euo pipefail

REGION="${REGION:-us-central1}"
PROJECT="${PROJECT:-cc-tunnel-local}"
REPO="${REPO:-cc-tunnel}"
IMAGE="${IMAGE:-cc-remote-agent}"
TAG="${TAG:-latest}"

REGISTRY="${REGION}-docker.pkg.dev"
FQIM="${REGISTRY}/${PROJECT}/${REPO}/${IMAGE}:${TAG}"
PROBE_IMAGE="${PROBE_IMAGE:-curlimages/curl:latest}"

log() { printf '[verify] %s\n' "$*"; }
fail() { printf '[verify][FAIL] %s\n' "$*" >&2; exit 1; }

log "== [1/4] Wait for Docker daemon to be ready"
for _ in $(seq 1 60); do
  if docker info >/dev/null 2>&1; then break; fi
  sleep 2
done
docker info >/dev/null 2>&1 || fail "docker daemon not ready after 120s"
log "docker daemon ready"

log "== [2/4] Bridge container -> metadata server reachability"
TOKEN_JSON="$(
  docker run --rm --network=bridge "${PROBE_IMAGE}" \
    -fsS \
    -H "Metadata-Flavor: Google" \
    "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"
)" || fail "metadata server unreachable from bridge container"

TOKEN="$(printf '%s' "${TOKEN_JSON}" | jq -er .access_token)" \
  || fail "metadata response did not contain access_token: ${TOKEN_JSON}"
log "got access token (length=${#TOKEN}) from bridge container"

log "== [3/4] docker login to ${REGISTRY} via oauth2accesstoken"
printf '%s' "${TOKEN}" \
  | docker login -u oauth2accesstoken --password-stdin "${REGISTRY}" \
  || fail "docker login to ${REGISTRY} failed"

log "== [4/4] docker pull ${FQIM}"
docker pull "${FQIM}" || fail "docker pull ${FQIM} failed"

log "ALL OK: bridge -> metadata reachable AND VM SA token can pull ${FQIM}"
