#!/bin/sh

set -eu

src="${POLE_CONFIG_SOURCE:-/config-src/pole-server.yaml}"
dst="${POLE_CONFIG_RENDERED:-/work/pole-server.yaml}"

: "${POD_IP:?POD_IP must be set}"
: "${POLE_CLUSTER_NAME:?POLE_CLUSTER_NAME must be set}"

mode="${POLE_MODE:-}"
if [ -z "${mode}" ]; then
  previous=""
  for argument in "$@"; do
    if [ "${previous}" = "--mode" ]; then
      mode="${argument}"
      break
    fi
    case "${argument}" in
      --mode=*) mode="${argument#--mode=}"; break ;;
    esac
    previous="${argument}"
  done
fi
mode="${mode:-all}"

require_value() {
  name="$1"
  value="$2"
  if [ -z "${value}" ]; then
    echo "${name} must be set for mode ${mode}" >&2
    exit 1
  fi
}

case "${mode}" in
  control-plane|server)
    require_value MYSQL_USER "${MYSQL_USER:-}"
    require_value MYSQL_PWD "${MYSQL_PWD:-}"
    require_value MYSQL_HOST "${MYSQL_HOST:-}"
    require_value POLE_AUTH_SALT "${POLE_AUTH_SALT:-}"
    require_value OTEL_COLLECTOR_GRPC_ENDPOINT "${OTEL_COLLECTOR_GRPC_ENDPOINT:-}"
    require_value OTEL_COLLECTOR_HTTP_ENDPOINT "${OTEL_COLLECTOR_HTTP_ENDPOINT:-}"
    ;;
  console)
    require_value MYSQL_USER "${MYSQL_USER:-}"
    require_value MYSQL_PWD "${MYSQL_PWD:-}"
    require_value MYSQL_HOST "${MYSQL_HOST:-}"
    require_value POLE_CONSOLE_JWT_SECRET "${POLE_CONSOLE_JWT_SECRET:-}"
    require_value GREPTIMEDB_HTTP_ENDPOINT "${GREPTIMEDB_HTTP_ENDPOINT:-}"
    ;;
  limiter-server)
    require_value POLE_LIMITER_CONTROL_PLANE_ADDRESS "${POLE_LIMITER_CONTROL_PLANE_ADDRESS:-}"
    ;;
  all|full)
    require_value MYSQL_USER "${MYSQL_USER:-}"
    require_value MYSQL_PWD "${MYSQL_PWD:-}"
    require_value MYSQL_HOST "${MYSQL_HOST:-}"
    require_value POLE_CONSOLE_JWT_SECRET "${POLE_CONSOLE_JWT_SECRET:-}"
    require_value POLE_AUTH_SALT "${POLE_AUTH_SALT:-}"
    require_value OTEL_COLLECTOR_GRPC_ENDPOINT "${OTEL_COLLECTOR_GRPC_ENDPOINT:-}"
    require_value OTEL_COLLECTOR_HTTP_ENDPOINT "${OTEL_COLLECTOR_HTTP_ENDPOINT:-}"
    require_value GREPTIMEDB_HTTP_ENDPOINT "${GREPTIMEDB_HTTP_ENDPOINT:-}"
    ;;
  *) echo "unsupported POLE_MODE: ${mode}" >&2; exit 1 ;;
esac

MYSQL_MAX_OPEN_CONNS="${MYSQL_MAX_OPEN_CONNS:-300}"
MYSQL_MAX_IDLE_CONNS="${MYSQL_MAX_IDLE_CONNS:-50}"
case "${MYSQL_MAX_OPEN_CONNS}" in ''|*[!0-9]*) echo 'MYSQL_MAX_OPEN_CONNS must be numeric' >&2; exit 1 ;; esac
case "${MYSQL_MAX_IDLE_CONNS}" in ''|*[!0-9]*) echo 'MYSQL_MAX_IDLE_CONNS must be numeric' >&2; exit 1 ;; esac

MYSQL_USER="${MYSQL_USER:-}"
MYSQL_PWD="${MYSQL_PWD:-}"
MYSQL_HOST="${MYSQL_HOST:-}"
POLE_CONSOLE_JWT_SECRET="${POLE_CONSOLE_JWT_SECRET:-}"
POLE_AUTH_SALT="${POLE_AUTH_SALT:-}"
OTEL_COLLECTOR_GRPC_ENDPOINT="${OTEL_COLLECTOR_GRPC_ENDPOINT:-127.0.0.1:4317}"
OTEL_COLLECTOR_HTTP_ENDPOINT="${OTEL_COLLECTOR_HTTP_ENDPOINT:-http://127.0.0.1:4318/v1/logs}"
GREPTIMEDB_HTTP_ENDPOINT="${GREPTIMEDB_HTTP_ENDPOINT:-http://127.0.0.1:4000}"
POLE_SERVER_ADDRESS="${POLE_SERVER_ADDRESS:-127.0.0.1:8090}"
POLE_LIMITER_SERVER_ADDRESS="${POLE_LIMITER_SERVER_ADDRESS:-127.0.0.1:8102}"
POLE_MCP_ENDPOINT="${POLE_MCP_ENDPOINT:-http://${POLE_SERVER_ADDRESS}/ai/mcp/v1/sse}"
POLE_LIMITER_CONTROL_PLANE_ADDRESS="${POLE_LIMITER_CONTROL_PLANE_ADDRESS:-127.0.0.1:8091}"
POLE_LIMITER_ADVERTISED_HOST="${POLE_LIMITER_ADVERTISED_HOST:-${POD_IP}}"
POLE_LIMITER_HTTP_BIND_IP="${POLE_LIMITER_HTTP_BIND_IP:-127.0.0.1}"
POLE_ENVIRONMENT="${POLE_ENVIRONMENT:-local}"
POLE_NODE_ROLE="${POLE_NODE_ROLE:-${mode}}"

validate_value() {
  name="$1"
  value="$2"
  case "${value}" in
    *"
"*|*"$(printf '\r')"*)
      echo "${name} must be single-line" >&2
      exit 1
      ;;
  esac
  if printf '%s' "${value}" | LC_ALL=C grep -q '[[:cntrl:]]'; then
    echo "${name} must not contain control characters" >&2
    exit 1
  fi
}

validate_value MYSQL_USER "${MYSQL_USER}"
validate_value MYSQL_PWD "${MYSQL_PWD}"
validate_value MYSQL_HOST "${MYSQL_HOST}"
validate_value POLE_CONSOLE_JWT_SECRET "${POLE_CONSOLE_JWT_SECRET}"
validate_value POLE_AUTH_SALT "${POLE_AUTH_SALT}"
validate_value POD_IP "${POD_IP}"
validate_value OTEL_COLLECTOR_GRPC_ENDPOINT "${OTEL_COLLECTOR_GRPC_ENDPOINT}"
validate_value OTEL_COLLECTOR_HTTP_ENDPOINT "${OTEL_COLLECTOR_HTTP_ENDPOINT}"
validate_value GREPTIMEDB_HTTP_ENDPOINT "${GREPTIMEDB_HTTP_ENDPOINT}"
validate_value POLE_CLUSTER_NAME "${POLE_CLUSTER_NAME}"
validate_value POLE_SERVER_ADDRESS "${POLE_SERVER_ADDRESS}"
validate_value POLE_LIMITER_SERVER_ADDRESS "${POLE_LIMITER_SERVER_ADDRESS}"
validate_value POLE_MCP_ENDPOINT "${POLE_MCP_ENDPOINT}"
validate_value POLE_LIMITER_CONTROL_PLANE_ADDRESS "${POLE_LIMITER_CONTROL_PLANE_ADDRESS}"
validate_value POLE_LIMITER_ADVERTISED_HOST "${POLE_LIMITER_ADVERTISED_HOST}"
validate_value POLE_LIMITER_HTTP_BIND_IP "${POLE_LIMITER_HTTP_BIND_IP}"
validate_value POLE_ENVIRONMENT "${POLE_ENVIRONMENT}"
validate_value POLE_NODE_ROLE "${POLE_NODE_ROLE}"

yaml_quote() {
  value="$1"
  escaped="$(printf '%s' "${value}" | sed 's/\\/\\\\/g; s/"/\\"/g')"
  printf '"%s"' "${escaped}"
}

export MYSQL_USER_YAML="$(yaml_quote "${MYSQL_USER}")"
export MYSQL_PWD_YAML="$(yaml_quote "${MYSQL_PWD}")"
export MYSQL_HOST_YAML="$(yaml_quote "${MYSQL_HOST}")"
export MYSQL_DSN_YAML="$(yaml_quote "${MYSQL_USER}:${MYSQL_PWD}@tcp(${MYSQL_HOST})/pole_server?loc=Local&parseTime=true")"
export POLE_CONSOLE_JWT_SECRET_YAML="$(yaml_quote "${POLE_CONSOLE_JWT_SECRET}")"
export POLE_AUTH_SALT_YAML="$(yaml_quote "${POLE_AUTH_SALT}")"
export POD_IP_YAML="$(yaml_quote "${POD_IP}")"
export OTEL_COLLECTOR_GRPC_ENDPOINT_YAML="$(yaml_quote "${OTEL_COLLECTOR_GRPC_ENDPOINT}")"
export OTEL_COLLECTOR_HTTP_ENDPOINT_YAML="$(yaml_quote "${OTEL_COLLECTOR_HTTP_ENDPOINT}")"
export GREPTIMEDB_HTTP_ENDPOINT_YAML="$(yaml_quote "${GREPTIMEDB_HTTP_ENDPOINT}")"
export POLE_CLUSTER_NAME_YAML="$(yaml_quote "${POLE_CLUSTER_NAME}")"
export POLE_SERVER_ADDRESS_YAML="$(yaml_quote "${POLE_SERVER_ADDRESS}")"
export POLE_LIMITER_SERVER_ADDRESS_YAML="$(yaml_quote "${POLE_LIMITER_SERVER_ADDRESS}")"
export POLE_MCP_ENDPOINT_YAML="$(yaml_quote "${POLE_MCP_ENDPOINT}")"
export POLE_LIMITER_CONTROL_PLANE_ADDRESS_YAML="$(yaml_quote "${POLE_LIMITER_CONTROL_PLANE_ADDRESS}")"
export POLE_LIMITER_ADVERTISED_HOST_YAML="$(yaml_quote "${POLE_LIMITER_ADVERTISED_HOST}")"
export POLE_LIMITER_HTTP_BIND_IP_YAML="$(yaml_quote "${POLE_LIMITER_HTTP_BIND_IP}")"
export POLE_ENVIRONMENT_YAML="$(yaml_quote "${POLE_ENVIRONMENT}")"
export POLE_NODE_ROLE_YAML="$(yaml_quote "${POLE_NODE_ROLE}")"

mkdir -p "$(dirname "${dst}")" /app/logs/runtime /app/.pole_data/observability/otel-events

sed \
  -e 's|dbUser: ##DB_USER##|dbUser: ${MYSQL_USER_YAML}|g' \
  -e 's|dbPwd: ##DB_PWD##|dbPwd: ${MYSQL_PWD_YAML}|g' \
  -e 's|dbAddr: ##DB_ADDR##|dbAddr: ${MYSQL_HOST_YAML}|g' \
  -e 's|dns: "${MYSQL_USER}:${MYSQL_PWD}@tcp(${MYSQL_HOST})/pole_server?loc=Local&parseTime=true" ##DB_DNS##|dns: ${MYSQL_DSN_YAML}|g' \
  -e 's|secretKey: polarismesh@2021|secretKey: ${POLE_CONSOLE_JWT_SECRET_YAML}|g' \
  -e 's|salt: polarismesh@2021|salt: ${POLE_AUTH_SALT_YAML}|g' \
  -e 's|# self_address: 127.0.0.1|self_address: ${POD_IP_YAML}|g' \
  -e 's|endpoint: http://127.0.0.1:4000|endpoint: ${GREPTIMEDB_HTTP_ENDPOINT_YAML}|g' \
  -e 's|address: 127.0.0.1:8090|address: ${POLE_SERVER_ADDRESS_YAML}|g' \
  -e 's|address: 127.0.0.1:8102|address: ${POLE_LIMITER_SERVER_ADDRESS_YAML}|g' \
  -e 's|endpoint: http://127.0.0.1:8090/ai/mcp/v1/sse|endpoint: ${POLE_MCP_ENDPOINT_YAML}|g' \
  -e 's|control-plane-address: 127.0.0.1:8091|control-plane-address: ${POLE_LIMITER_CONTROL_PLANE_ADDRESS_YAML}|g' \
  -e 's|advertised-host: 127.0.0.1|advertised-host: ${POLE_LIMITER_ADVERTISED_HOST_YAML}|g' \
  -e 's|ip: 127.0.0.1|ip: ${POLE_LIMITER_HTTP_BIND_IP_YAML}|g' \
  -e 's|endpoint: 127.0.0.1:4317|endpoint: ${OTEL_COLLECTOR_GRPC_ENDPOINT_YAML}|g' \
  -e 's|logsEndpoint: http://127.0.0.1:4318/v1/logs|logsEndpoint: ${OTEL_COLLECTOR_HTTP_ENDPOINT_YAML}|g' \
  -e 's|maxOpenConns: 300|maxOpenConns: ${MYSQL_MAX_OPEN_CONNS}|g' \
  -e 's|maxIdleConns: 50|maxIdleConns: ${MYSQL_MAX_IDLE_CONNS}|g' \
  -e 's|cluster: local-docker|cluster: ${POLE_CLUSTER_NAME_YAML}|g' \
  -e 's|environment: local|environment: ${POLE_ENVIRONMENT_YAML}|g' \
  -e 's|nodeRole: control-plane|nodeRole: ${POLE_NODE_ROLE_YAML}|g' \
  "${src}" >"${dst}"

# The source file is maintained outside this image. Fail closed when its shape
# changes instead of silently starting with a default credential or endpoint.
for required in \
  '${MYSQL_USER_YAML}' \
  '${MYSQL_PWD_YAML}' \
  '${MYSQL_HOST_YAML}' \
  '${MYSQL_DSN_YAML}' \
  '${POLE_CONSOLE_JWT_SECRET_YAML}' \
  '${POLE_AUTH_SALT_YAML}' \
  '${POD_IP_YAML}' \
  '${GREPTIMEDB_HTTP_ENDPOINT_YAML}' \
  '${OTEL_COLLECTOR_GRPC_ENDPOINT_YAML}' \
  '${OTEL_COLLECTOR_HTTP_ENDPOINT_YAML}' \
  '${POLE_CLUSTER_NAME_YAML}' \
  '${POLE_SERVER_ADDRESS_YAML}' \
  '${POLE_LIMITER_SERVER_ADDRESS_YAML}' \
  '${POLE_MCP_ENDPOINT_YAML}' \
  '${POLE_LIMITER_CONTROL_PLANE_ADDRESS_YAML}' \
  '${POLE_LIMITER_ADVERTISED_HOST_YAML}' \
  '${POLE_LIMITER_HTTP_BIND_IP_YAML}' \
  '${POLE_ENVIRONMENT_YAML}' \
  '${POLE_NODE_ROLE_YAML}'
do
  if ! grep -Fq "${required}" "${dst}"; then
    echo "rendered configuration is missing required placeholder: ${required}" >&2
    exit 1
  fi
done

if grep -Eq 'db(User|Pwd|Addr): ##DB_|##DB_DNS##|polarismesh@2021|# self_address: 127\.0\.0\.1|advertised-host: 127\.0\.0\.1' "${dst}"; then
  echo 'rendered configuration still contains an unsafe source default' >&2
  exit 1
fi

exec /app/pole-server "$@"
