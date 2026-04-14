#!/usr/bin/env bash
# dev-stack.sh — Server lifecycle management for development.
#
# Usage:
#   scripts/dev-stack.sh              # restart + seed (most common)
#   scripts/dev-stack.sh up           # start services
#   scripts/dev-stack.sh down         # stop and remove volumes
#   scripts/dev-stack.sh restart      # down then up
#   scripts/dev-stack.sh seed         # seed all fixtures
#   scripts/dev-stack.sh status       # show service health
set -euo pipefail

FIXTURES_DIR="fixtures"
DECREE_CMD="${DECREE_CMD:-decree}"

cmd_up() {
  echo "Starting services..."
  docker compose up -d --build --wait service
  echo "Services ready."
}

cmd_down() {
  echo "Stopping services..."
  docker compose down -v
  echo "Services stopped."
}

cmd_seed() {
  echo "Seeding fixtures..."
  for f in "$FIXTURES_DIR"/*.yaml; do
    name=$(basename "$f" .yaml)
    if [[ "$name" == "draft" ]]; then
      # Draft fixture has no --auto-publish (by design)
      "$DECREE_CMD" seed "$f" --subject admin 2>&1 && echo "  ✓ $name" || echo "  ○ $name (expected — unpublished schema)"
    else
      "$DECREE_CMD" seed "$f" --auto-publish --subject admin 2>&1 && echo "  ✓ $name" || echo "  ✗ $name FAILED"
    fi
  done
  echo "Seeding complete."
}

cmd_status() {
  echo "=== Services ==="
  docker compose ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "No services running."
  echo ""
  echo "=== Health ==="
  curl -sf http://localhost:8080/v1/schemas -H "x-subject: admin" > /dev/null 2>&1 \
    && echo "✓ REST API healthy (localhost:8080)" \
    || echo "✗ REST API not responding"
}

ACTION="${1:-}"

case "$ACTION" in
  up)      cmd_up ;;
  down)    cmd_down ;;
  restart) cmd_down; cmd_up ;;
  seed)    cmd_seed ;;
  status)  cmd_status ;;
  "")      cmd_down; cmd_up; cmd_seed ;;
  *)       echo "Usage: $0 [up|down|restart|seed|status]"; exit 1 ;;
esac
