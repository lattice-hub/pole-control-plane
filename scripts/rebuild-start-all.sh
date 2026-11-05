#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

POLE_BIN_PATH="${POLE_BIN_PATH:-/tmp/pole-control-plane}"
POLE_CONFIG_PATH="${POLE_CONFIG_PATH:-/tmp/pole-server-all.yaml}"
POLE_STOP_EXISTING="${POLE_STOP_EXISTING:-1}"
# When set to 1, kill whatever listens on the target ports without verifying it
# is a Pole process. Useful in restricted environments where process metadata
# (ps/lsof command name) is unavailable.
POLE_FORCE_KILL="${POLE_FORCE_KILL:-0}"
POLE_START_MODE="${POLE_START_MODE:-all}"
POLE_DETACH="${POLE_DETACH:-0}"
POLE_DETACH_BACKEND="${POLE_DETACH_BACKEND:-auto}"
POLE_DETACHED_CHILD="${POLE_DETACHED_CHILD:-0}"
POLE_TMUX_SESSION="${POLE_TMUX_SESSION:-pole-control-plane}"
POLE_LOG_PATH="${POLE_LOG_PATH:-/tmp/pole-control-plane-all.log}"

MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PWD="${MYSQL_PWD:-123456}"
MYSQL_HOST="${MYSQL_HOST:-127.0.0.1:3306}"

export MYSQL_USER MYSQL_PWD MYSQL_HOST

log() {
  printf '[rebuild-start-all] %s\n' "$*"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf '[rebuild-start-all] missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

usage() {
  cat <<'EOF'
Usage:
  ./scripts/rebuild-start-all.sh [options]

Options:
  --detach[=auto|tmux|nohup]  Run the full rebuild/start flow in a detached session.
                              Defaults to tmux when available, then setsid/nohup.
  --foreground                Run in the current terminal and exec the all-mode server.
  --no-stop-existing          Do not stop existing Pole processes on 8080/8090/8101/8102.
  --force-kill                Kill listeners on product ports even if process metadata is hidden.
  -h, --help                  Show this help.

Environment:
  MYSQL_USER, MYSQL_PWD, MYSQL_HOST
  POLE_BIN_PATH, POLE_CONFIG_PATH
  POLE_START_MODE=all|full
  POLE_TMUX_SESSION, POLE_LOG_PATH
  POLE_STOP_EXISTING=0, POLE_FORCE_KILL=1
EOF
}

parse_args() {
  local arg
  for arg in "$@"; do
    case "${arg}" in
      --detach)
        POLE_DETACH=1
        POLE_DETACH_BACKEND=auto
        ;;
      --detach=*)
        POLE_DETACH=1
        POLE_DETACH_BACKEND="${arg#--detach=}"
        ;;
      --foreground)
        POLE_DETACH=0
        ;;
      --no-stop-existing)
        POLE_STOP_EXISTING=0
        ;;
      --force-kill)
        POLE_FORCE_KILL=1
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        printf '[rebuild-start-all] unknown argument: %s\n' "${arg}" >&2
        usage >&2
        exit 1
        ;;
    esac
  done
}

detached_child_command() {
  printf 'cd %q && ' "${ROOT_DIR}"
  printf 'MYSQL_USER=%q MYSQL_PWD=%q MYSQL_HOST=%q ' "${MYSQL_USER}" "${MYSQL_PWD}" "${MYSQL_HOST}"
  printf 'POLE_BIN_PATH=%q POLE_CONFIG_PATH=%q ' "${POLE_BIN_PATH}" "${POLE_CONFIG_PATH}"
  printf 'POLE_STOP_EXISTING=%q POLE_FORCE_KILL=%q POLE_START_MODE=%q ' "${POLE_STOP_EXISTING}" "${POLE_FORCE_KILL}" "${POLE_START_MODE}"
  printf 'POLE_DETACH=0 POLE_DETACHED_CHILD=1 '
  printf '%q --foreground' "${ROOT_DIR}/scripts/rebuild-start-all.sh"
}

detach_and_exit() {
  local backend child_cmd

  if [[ "${POLE_DETACH}" != "1" || "${POLE_DETACHED_CHILD}" == "1" ]]; then
    return
  fi

  case "${POLE_DETACH_BACKEND}" in
    auto|tmux|nohup) ;;
    *)
      printf '[rebuild-start-all] invalid detach backend: %s\n' "${POLE_DETACH_BACKEND}" >&2
      exit 1
      ;;
  esac

  backend="${POLE_DETACH_BACKEND}"
  if [[ "${backend}" == "auto" ]]; then
    if command -v tmux >/dev/null 2>&1; then
      backend="tmux"
    else
      backend="nohup"
    fi
  fi

  child_cmd="$(detached_child_command)"

  case "${backend}" in
    tmux)
      require_cmd tmux
      if tmux has-session -t "${POLE_TMUX_SESSION}" 2>/dev/null; then
        log "kill existing tmux session: ${POLE_TMUX_SESSION}"
        tmux kill-session -t "${POLE_TMUX_SESSION}"
      fi
      log "detach rebuild/start into tmux session: ${POLE_TMUX_SESSION}"
      tmux new-session -d -s "${POLE_TMUX_SESSION}" -c "${ROOT_DIR}" "${child_cmd}"
      log "watch logs: tmux capture-pane -pt ${POLE_TMUX_SESSION}:0 -S -200"
      ;;
    nohup)
      log "detach rebuild/start with setsid/nohup, log=${POLE_LOG_PATH}"
      mkdir -p "$(dirname "${POLE_LOG_PATH}")"
      if command -v setsid >/dev/null 2>&1; then
        setsid bash -lc "${child_cmd}" >>"${POLE_LOG_PATH}" 2>&1 </dev/null &
      else
        nohup bash -lc "${child_cmd}" >>"${POLE_LOG_PATH}" 2>&1 </dev/null &
      fi
      log "watch logs: tail -f ${POLE_LOG_PATH}"
      ;;
  esac

  exit 0
}

stop_existing_pole_processes() {
  if [[ "${POLE_STOP_EXISTING}" != "1" ]]; then
    log "skip stopping existing Pole processes"
    return
  fi

  if ! command -v lsof >/dev/null 2>&1; then
    log "lsof not found, skip port cleanup"
    return
  fi

  local ports=(8080 8090 8101 8102)
  local pid command_name killed_any=0

  # Resolve the process name for a PID without relying on `ps`, which is blocked
  # in some sandboxes. `lsof -p <pid> -F c` prints a line like `cmd<name>`.
  pole_process_name() {
    local target_pid="$1" line name=""
    while IFS= read -r line; do
      case "${line}" in
        c*) name="${line#c}" ;;
      esac
    done < <(lsof -nP -p "${target_pid}" -F c 2>/dev/null || true)
    printf '%s' "${name}"
  }

  for port in "${ports[@]}"; do
    while IFS= read -r pid; do
      [[ -z "${pid}" ]] && continue
      command_name="$(pole_process_name "${pid}")"
      if [[ "${POLE_FORCE_KILL}" == "1" \
            || "${command_name}" == *"pole-control-plane"* \
            || "${command_name}" == *"pole-server"* \
            || "${command_name}" == "pole-cont"* ]]; then
        log "stop existing Pole process on port ${port}: pid=${pid} name=${command_name:-unknown}"
        if ! kill "${pid}" 2>/dev/null; then
          printf '[rebuild-start-all] failed to stop pid=%s (operation not permitted?). stop it manually, then rerun.\n' "${pid}" >&2
          exit 1
        fi
        killed_any=1
      else
        printf '[rebuild-start-all] port %s is occupied by a non-Pole process: pid=%s name=%s\n' "${port}" "${pid}" "${command_name:-unknown}" >&2
        printf '[rebuild-start-all] if this is a Pole process whose name could not be resolved, rerun with POLE_FORCE_KILL=1.\n' >&2
        exit 1
      fi
    done < <(lsof -nP -tiTCP:"${port}" -sTCP:LISTEN 2>/dev/null || true)
  done

  [[ "${killed_any}" == "1" ]] && sleep 2 || true

  # Confirm the ports are actually free before continuing.
  for port in "${ports[@]}"; do
    if lsof -nP -tiTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1; then
      printf '[rebuild-start-all] port %s is still in use after stop attempt. stop the owning process manually, then rerun.\n' "${port}" >&2
      exit 1
    fi
  done
}

build_console_web() {
  log "build console web"
  "${ROOT_DIR}/scripts/build-console-assets.sh" release
}

build_control_plane_binary() {
  local version build_date package

  version="$(cat "${ROOT_DIR}/version" 2>/dev/null || printf 'dev')"
  build_date="$(date '+%Y%m%d.%H%M%S')"
  package="github.com/pole-io/pole-server/pkg/common/version"

  log "build control-plane binary: ${POLE_BIN_PATH}"
  (
    cd "${ROOT_DIR}"
    CGO_ENABLED="${CGO_ENABLED:-0}" go build -tags consoleassets -o "${POLE_BIN_PATH}" \
      -ldflags="-X ${package}.Version=${version} -X ${package}.BuildDate=${build_date}" .
  )
}

generate_all_config() {
  local log_config apiserver_config

  log_config="${ROOT_DIR}/deploy/conf/pole-log.yaml"
  apiserver_config="${ROOT_DIR}/deploy/conf/pole-apiserver.yaml"

  log "generate all-mode config: ${POLE_CONFIG_PATH}"
  cp "${ROOT_DIR}/deploy/conf/pole-server.yaml" "${POLE_CONFIG_PATH}"

  POLE_LOG_CONFIG="${log_config}" \
  POLE_APISERVER_CONFIG="${apiserver_config}" \
  perl -0pi -e '
    s|logger: \./conf/pole-log\.yaml|logger: $ENV{POLE_LOG_CONFIG}|g;
    s|apiservers: \./conf/pole-apiserver\.yaml|apiservers: $ENV{POLE_APISERVER_CONFIG}|g;
    s|dbUser: ##DB_USER##|dbUser: $ENV{MYSQL_USER}|g;
    s|dbPwd: ##DB_PWD##|dbPwd: $ENV{MYSQL_PWD}|g;
    s|dbAddr: ##DB_ADDR##|dbAddr: $ENV{MYSQL_HOST}|g;
  ' "${POLE_CONFIG_PATH}"
}

start_all_mode() {
  log "start ${POLE_START_MODE} mode"
  log "config=${POLE_CONFIG_PATH}"
  log "mysql=${MYSQL_USER}@${MYSQL_HOST}"

  cd "${ROOT_DIR}"
  exec "${POLE_BIN_PATH}" start -c "${POLE_CONFIG_PATH}" --mode "${POLE_START_MODE}"
}

main() {
  parse_args "$@"

  case "${POLE_START_MODE}" in
    all|full) ;;
    *)
      printf '[rebuild-start-all] POLE_START_MODE must be all or full, got: %s\n' "${POLE_START_MODE}" >&2
      exit 1
      ;;
  esac

  detach_and_exit

  require_cmd npm
  require_cmd go
  require_cmd perl

  stop_existing_pole_processes
  build_console_web
  build_control_plane_binary
  generate_all_config
  start_all_mode
}

main "$@"
