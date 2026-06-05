#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${REPO_URL:-https://github.com/litentry/subscan-essentials.git}"
BRANCH="${BRANCH:-crossagent}"
DEPLOY_ROOT="${DEPLOY_ROOT:-/opt/subscan}"
SRC_DIR="${SRC_DIR:-$DEPLOY_ROOT/subscan-essentials-ci}"
NETWORK="${NETWORK:-subscan-essentials_subscan_net}"

MAIN_CONTAINER="${MAIN_CONTAINER:-subscan-essentials-subscan-api-1}"
CROSSAGENT_CONTAINER="${CROSSAGENT_CONTAINER:-subscan-essentials-crossagent-subscan-api-crossagent-1}"
OBSERVER_CONTAINER="${OBSERVER_CONTAINER:-subscan-essentials-subscan-observer-1}"
WORKER_CONTAINER="${WORKER_CONTAINER:-subscan-essentials-subscan-worker-1}"
MAIN_IMAGE="${MAIN_IMAGE:-subscan/api}"
CROSSAGENT_IMAGE="${CROSSAGENT_IMAGE:-subscan/api:crossagent}"
RUN_OMNIBRIDGE_BACKFILL="${RUN_OMNIBRIDGE_BACKFILL:-1}"

MYSQL_PASSWORD="${MYSQL_PASSWORD:-subscan2024heima}"
CHAIN_WS_ENDPOINT="${CHAIN_WS_ENDPOINT:-wss://rpc.heima-parachain.heima.network}"
ETH_RPC="${ETH_RPC:-https://rpc.heima-parachain.heima.network}"
NETWORK_NODE="${NETWORK_NODE:-heima}"

run_sudo() {
  sudo -n "$@"
}

ensure_source() {
  run_sudo mkdir -p "$DEPLOY_ROOT"
  run_sudo chown "$(id -u):$(id -g)" "$DEPLOY_ROOT"

  if [[ ! -d "$SRC_DIR/.git" ]]; then
    rm -rf "$SRC_DIR.tmp"
    git clone --branch "$BRANCH" --single-branch "$REPO_URL" "$SRC_DIR.tmp"
    touch "$SRC_DIR.tmp/.ci-deploy-owned"
    mv "$SRC_DIR.tmp" "$SRC_DIR"
    return
  fi

  if [[ ! -f "$SRC_DIR/.ci-deploy-owned" ]]; then
    echo "Refusing to update $SRC_DIR because it is not marked as CI-owned." >&2
    exit 1
  fi

  git -C "$SRC_DIR" fetch --prune origin "$BRANCH"
  git -C "$SRC_DIR" reset --hard
  git -C "$SRC_DIR" clean -fdx -e .ci-deploy-owned
  git -C "$SRC_DIR" checkout -B "$BRANCH" "origin/$BRANCH"
  git -C "$SRC_DIR" reset --hard "origin/$BRANCH"
  git -C "$SRC_DIR" clean -fdx -e .ci-deploy-owned
}

ensure_supporting_services() {
  run_sudo docker start subscan-essentials-mysql-1 >/dev/null
  run_sudo docker start subscan-essentials-redis-1 >/dev/null
}

build_images() {
  run_sudo docker build --pull -t "$CROSSAGENT_IMAGE" "$SRC_DIR"
  run_sudo docker tag "$CROSSAGENT_IMAGE" "$MAIN_IMAGE"
}

replace_api_container() {
  local name="$1"
  local image="$2"
  local host_port="$3"
  local mysql_host="$4"
  local redis_addr="$5"

  if run_sudo docker ps -a --format '{{.Names}}' | grep -Fxq "$name"; then
    run_sudo docker rm -f "$name" >/dev/null
  fi

  run_sudo docker run -d \
    --name "$name" \
    --restart always \
    --network "$NETWORK" \
    -p "$host_port:4399" \
    -e "MYSQL_HOST=$mysql_host" \
    -e "MYSQL_PASS=$MYSQL_PASSWORD" \
    -e MYSQL_USER=root \
    -e MYSQL_DB=subscan \
    -e "REDIS_ADDR=$redis_addr" \
    -e "CHAIN_WS_ENDPOINT=$CHAIN_WS_ENDPOINT" \
    -e "ETH_RPC=$ETH_RPC" \
    -e "NETWORK_NODE=$NETWORK_NODE" \
    -e DEPLOY_ENV=prod \
    "$image" >/dev/null
}

replace_runtime_container() {
  local name="$1"
  local image="$2"
  local mysql_host="$3"
  local redis_addr="$4"
  shift 4

  if run_sudo docker ps -a --format '{{.Names}}' | grep -Fxq "$name"; then
    run_sudo docker rm -f "$name" >/dev/null
  fi

  run_sudo docker run -d \
    --name "$name" \
    --restart always \
    --network "$NETWORK" \
    -e "MYSQL_HOST=$mysql_host" \
    -e "MYSQL_PASS=$MYSQL_PASSWORD" \
    -e MYSQL_USER=root \
    -e MYSQL_DB=subscan \
    -e "REDIS_ADDR=$redis_addr" \
    -e "CHAIN_WS_ENDPOINT=$CHAIN_WS_ENDPOINT" \
    -e "ETH_RPC=$ETH_RPC" \
    -e "NETWORK_NODE=$NETWORK_NODE" \
    -e DEPLOY_ENV=prod \
    "$image" "$@" >/dev/null
}

run_omnibridge_backfill() {
  if [[ "$RUN_OMNIBRIDGE_BACKFILL" != "1" ]]; then
    echo "Skipping OmniBridge backfill because RUN_OMNIBRIDGE_BACKFILL=$RUN_OMNIBRIDGE_BACKFILL."
    return
  fi

  run_sudo docker run --rm \
    --network "$NETWORK" \
    -e MYSQL_HOST=mysql \
    -e "MYSQL_PASS=$MYSQL_PASSWORD" \
    -e MYSQL_USER=root \
    -e MYSQL_DB=subscan \
    -e REDIS_ADDR=redis:6379 \
    -e "CHAIN_WS_ENDPOINT=$CHAIN_WS_ENDPOINT" \
    -e "ETH_RPC=$ETH_RPC" \
    -e "NETWORK_NODE=$NETWORK_NODE" \
    -e DEPLOY_ENV=prod \
    "$MAIN_IMAGE" plugin balance BackfillOmniBridgeTransfers
}

wait_for_api() {
  local host_port="$1"
  local body=""

  for _ in {1..30}; do
    body="$(curl -fsS -X POST "http://127.0.0.1:$host_port/api/plugin/evm/accounts" \
      -H 'content-type: application/json' \
      --data '{"row":1}' 2>/dev/null || true)"
    if [[ "$body" == *'"code":0'* ]]; then
      echo "API on $host_port is healthy."
      return
    fi
    sleep 2
  done

  echo "API on $host_port did not become healthy." >&2
  run_sudo docker ps --filter "publish=$host_port"
  exit 1
}

main() {
  ensure_source
  ensure_supporting_services
  build_images

  replace_api_container "$MAIN_CONTAINER" "$MAIN_IMAGE" 4399 mysql redis:6379
  wait_for_api 4399
  run_omnibridge_backfill
  replace_runtime_container "$OBSERVER_CONTAINER" "$MAIN_IMAGE" mysql redis:6379 start subscribe
  replace_runtime_container "$WORKER_CONTAINER" "$MAIN_IMAGE" mysql redis:6379 start worker

  replace_api_container "$CROSSAGENT_CONTAINER" "$CROSSAGENT_IMAGE" 4599 subscan-essentials-mysql-1 subscan-essentials-redis-1:6379
  wait_for_api 4599

  run_sudo docker ps \
    --filter "name=$MAIN_CONTAINER" \
    --filter "name=$CROSSAGENT_CONTAINER" \
    --filter "name=$OBSERVER_CONTAINER" \
    --filter "name=$WORKER_CONTAINER" \
    --format '{{.Names}} {{.Image}} {{.Status}} {{.Ports}}'
}

main "$@"
