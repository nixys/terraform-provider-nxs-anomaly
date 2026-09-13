#!/usr/bin/env bash
# Run provider acceptance tests against a live nxs-anomaly instance.
#
# If NXS_ANOMALY_URL is absent, start PostgreSQL in Docker and run a local
# nxs-anomaly process, execute tests, then clean up.
#
# If NXS_ANOMALY_URL is supplied, use that instance directly.
#
# Optional environment variables:
#   NXS_ANOMALY_URL          Existing instance URL; skips automatic setup
#   NXS_ANOMALY_API_KEY      API key; defaults to ci-test-key for automatic setup
#   NXS_ANOMALY_REPO         HTTPS repository URL to clone
#   NXS_ANOMALY_BINARY       Existing service binary; skips building
#   PG_IMAGE                PostgreSQL image (default: postgres:17-alpine)
#   PG_PORT                 PostgreSQL host port (default: 55433)
#   API_PORT                Service host port (default: 58080)
#   GOCACHE / GOMODCACHE     Go caches
set -euo pipefail
export TF_ACC=1

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

export GOCACHE="${GOCACHE:-/tmp/go-build}"
export GOMODCACHE="${GOMODCACHE:-/tmp/go-mod}"

# Run tests directly when an external instance is configured.

if [[ -n "${NXS_ANOMALY_URL:-}" ]]; then
  echo "==> Using external nxs-anomaly at ${NXS_ANOMALY_URL}"
  go test ./internal/provider/... -run "^TestAcc" -v -count=1 -timeout 10m
  exit 0
fi

# ── Auto-setup: PostgreSQL + nxs-anomaly ──────────────────────────────────────

PG_IMAGE="${PG_IMAGE:-postgres:17-alpine}"
PG_PORT="${PG_PORT:-55433}"
API_PORT="${API_PORT:-58080}"
PG_DB="nxs_tf_test"
PG_USER="nxs_tf"
PG_PASS="nxs_tf_secret"
PG_CONT="nxs-tf-postgres-$$"
ANOMALY_CONT=""
ANOMALY_PID=""
CI_TEST_API_KEY="${NXS_ANOMALY_API_KEY:-ci-test-key}"

cleanup() {
  echo "==> Cleanup"
  [[ -n "${ANOMALY_PID}" ]] && kill "${ANOMALY_PID}" 2>/dev/null || true
  [[ -n "${ANOMALY_CONT}" ]] && docker rm -f "${ANOMALY_CONT}" 2>/dev/null || true
  docker rm -f "${PG_CONT}" 2>/dev/null || true
}
trap cleanup EXIT

# ── 1. PostgreSQL ─────────────────────────────────────────────────────────────

echo "==> Starting PostgreSQL (${PG_IMAGE}) on port ${PG_PORT}"
docker run -d \
  --name "${PG_CONT}" \
  -e POSTGRES_DB="${PG_DB}" \
  -e POSTGRES_USER="${PG_USER}" \
  -e POSTGRES_PASSWORD="${PG_PASS}" \
  -p "127.0.0.1:${PG_PORT}:5432" \
  "${PG_IMAGE}" >/dev/null

echo -n "==> Waiting for PostgreSQL "
for _ in $(seq 1 60); do
  if docker exec "${PG_CONT}" pg_isready -U "${PG_USER}" -d "${PG_DB}" >/dev/null 2>&1; then
    echo " ready"
    break
  fi
  echo -n "."
  sleep 1
done
docker exec "${PG_CONT}" pg_isready -U "${PG_USER}" -d "${PG_DB}" >/dev/null

DATABASE_URL="postgres://${PG_USER}:${PG_PASS}@127.0.0.1:${PG_PORT}/${PG_DB}?sslmode=disable"

# ── 2. nxs-anomaly binary ─────────────────────────────────────────────────────

NXS_BINARY="${NXS_ANOMALY_BINARY:-}"

if [[ -z "${NXS_BINARY}" ]]; then
  REPO_URL="${NXS_ANOMALY_REPO:-}"
  if [[ -z "${REPO_URL}" ]]; then
    # In GitLab CI, use CI_JOB_TOKEN to access the service repository.
    if [[ -n "${CI_SERVER_URL:-}" && -n "${CI_JOB_TOKEN:-}" ]]; then
      SERVER="${CI_SERVER_URL#https://}"
      SERVER="${SERVER#http://}"
      REPO_URL="https://gitlab-ci-token:${CI_JOB_TOKEN}@${SERVER}/teamx/nxs-anomaly-group/nxs-anomaly.git"
    else
      REPO_URL="https://github.com/nixys/nxs-anomaly.git"
    fi
  fi

  CLONE_DIR="/tmp/nxs-anomaly-src-$$"
  echo "==> Cloning the nxs-anomaly service repository"
  git clone --depth=1 "${REPO_URL}" "${CLONE_DIR}"

  echo "==> Building nxs-anomaly"
  NXS_BINARY="/tmp/nxs-anomaly-bin-$$"
  (
    cd "${CLONE_DIR}"
    GOCACHE="${GOCACHE}" GOMODCACHE="${GOMODCACHE}" \
      go build -o "${NXS_BINARY}" ./cmd/nxs-anomaly
  )
  rm -rf "${CLONE_DIR}"
fi

echo "==> nxs-anomaly binary: ${NXS_BINARY}"

# ── 3. Start nxs-anomaly ─────────────────────────────────────────────────────

echo "==> Starting nxs-anomaly"
NXS_ANOMALY_DB_DSN="${DATABASE_URL}" \
NXS_ANOMALY_API_KEY="${CI_TEST_API_KEY}" \
NXS_ANOMALY_ADDR="127.0.0.1:${API_PORT}" \
  "${NXS_BINARY}" serve &
ANOMALY_PID=$!

echo -n "==> Waiting for nxs-anomaly "
for _ in $(seq 1 30); do
  if curl -sf http://127.0.0.1:${API_PORT}/health >/dev/null 2>&1; then
    echo " ready"
    break
  fi
  echo -n "."
  sleep 1
  # Check the process hasn't crashed
  kill -0 "${ANOMALY_PID}" 2>/dev/null || { echo ""; echo "ERROR: nxs-anomaly exited early"; exit 1; }
done
curl -sf http://127.0.0.1:${API_PORT}/health >/dev/null

# ── 4. Acceptance tests ───────────────────────────────────────────────────────

echo "==> Running acceptance tests"
NXS_ANOMALY_URL="http://127.0.0.1:${API_PORT}" \
NXS_ANOMALY_API_KEY="${CI_TEST_API_KEY}" \
  go test ./internal/provider/... -run "^TestAcc" -v -count=1 -timeout 10m

echo "==> All acceptance tests passed"
