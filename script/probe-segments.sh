#!/usr/bin/env bash
#
# probe-segments.sh — 抽样验证 CIDR 段健康度，把通过的段写回 ip.txt（v2）
#
# 工作流（一键闭环）:
#   读 ip.txt.full  →  每段随机抽 N 个 IP 并发测试
#                  →  通过率 >= 阈值的段写到 ip.txt
#
# 用法:
#   ./probe-segments.sh                          # 默认: 读 ../ip.txt.full → 写 ../ip.txt
#   ./probe-segments.sh in.txt                   # 指定输入
#   ./probe-segments.sh in.txt out.txt           # 指定输入和输出
#
# 环境变量:
#   SAMPLES=10        每段抽样数
#   THRESHOLD=50      通过率阈值（百分比，>= 视为有效段）
#   PARALLEL=10       段内并发数
#   TIMEOUT=4         单次 curl 超时秒数
#   VERBOSE=0         设 1 显示每个抽样 IP 的 PASS/FAIL 明细
#   DRY_RUN=0         设 1 只打印结果不写文件
#
# 状态标记:
#   OK    pass >= 阈值          段健康
#   WARN  0 < pass < 阈值       部分可用
#   DEAD  pass = 0              段在你网络下全死
#
# 通过判定:
#   HTTP 状态码 200/301/302/403 视为 PASS
#   （403 是 CF 对 HEAD 请求的预期响应，TLS+HTTP 链路其实是通的，
#    下载阶段是 GET 不会 403）
#
# 失败保护:
#   如果没有任何段达到阈值，不会覆盖输出文件（保留旧版），exit 1
#

set -u

URL='https://speed.cloudflare.com/__down?bytes=10000'
HOST='speed.cloudflare.com'

SAMPLES="${SAMPLES:-10}"
THRESHOLD="${THRESHOLD:-50}"
PARALLEL="${PARALLEL:-10}"
TIMEOUT="${TIMEOUT:-4}"
VERBOSE="${VERBOSE:-0}"
DRY_RUN="${DRY_RUN:-0}"

SCRIPT_DIR="$(dirname "$0")"
INPUT="${1:-$SCRIPT_DIR/../ip.txt.full}"
OUTPUT="${2:-$SCRIPT_DIR/../ip.txt}"

[ -f "$INPUT" ] || { echo "输入文件找不到: $INPUT" >&2; exit 1; }

ip2int() {
  local IFS=.
  read -r a b c d <<< "$1"
  echo $(( (a << 24) | (b << 16) | (c << 8) | d ))
}

int2ip() {
  local n=$1
  printf "%d.%d.%d.%d\n" $((n >> 24 & 255)) $((n >> 16 & 255)) $((n >> 8 & 255)) $((n & 255))
}

probe_one() {
  local ip="$1"
  local code
  code=$(curl -w "%{http_code}" -o /dev/null -s -m "$TIMEOUT" -I \
    --resolve "${HOST}:443:${ip}" "$URL" 2>/dev/null)
  case "$code" in
    200|301|302|403) echo "PASS $ip ($code)" ;;
    *)               echo "FAIL $ip ($code)" ;;
  esac
}
export -f probe_one
export URL HOST TIMEOUT

echo "输入: $INPUT"
echo "输出: $OUTPUT$([ "$DRY_RUN" = "1" ] && echo "  (DRY_RUN: 不写文件)")"
echo "参数: SAMPLES=$SAMPLES THRESHOLD=${THRESHOLD}% PARALLEL=$PARALLEL TIMEOUT=${TIMEOUT}s"
echo ""

passed_cidrs=()

while IFS= read -r cidr; do
  cidr="${cidr%$'\r'}"
  [ -z "$cidr" ] && continue
  case "$cidr" in \#*) continue;; esac

  ip="${cidr%%/*}"
  prefix="${cidr##*/}"
  net_int=$(ip2int "$ip")
  host_count=$(( 1 << (32 - prefix) ))

  if [ "$host_count" -le 2 ]; then
    n_samples=$host_count
    range=$host_count
    base_offset=0
  elif [ "$host_count" -lt $((SAMPLES + 2)) ]; then
    n_samples=$((host_count - 2))
    range=$((host_count - 2))
    base_offset=1
  else
    n_samples=$SAMPLES
    range=$((host_count - 2))
    base_offset=1
  fi

  samples=()
  for ((i=0; i<n_samples; i++)); do
    offset=$(( base_offset + ((RANDOM << 15 | RANDOM)) % range ))
    samples+=("$(int2ip $((net_int + offset)))")
  done

  results=$(printf '%s\n' "${samples[@]}" \
    | xargs -P "$PARALLEL" -I {} bash -c 'probe_one "$@"' _ {})

  pass=$(printf '%s\n' "$results" | grep -c '^PASS' || true)
  pct=$(( pass * 100 / n_samples ))

  if [ "$pct" -ge "$THRESHOLD" ]; then
    mark="OK  "
    passed_cidrs+=("$cidr")
  elif [ "$pass" -gt 0 ]; then
    mark="WARN"
  else
    mark="DEAD"
  fi

  printf "%s  %-22s pass=%d/%d (%d%%)\n" "$mark" "$cidr" "$pass" "$n_samples" "$pct"

  if [ "$VERBOSE" = "1" ]; then
    printf '%s\n' "$results" | sed 's/^/      /'
  fi
done < "$INPUT"

echo ""

if [ "${#passed_cidrs[@]}" -eq 0 ]; then
  echo "[WARN] 没有段达到 ${THRESHOLD}% 阈值，未修改 $OUTPUT" >&2
  echo "       建议: 降低 THRESHOLD（如 30）、增加 SAMPLES、或检查网络是否被运营商限速" >&2
  exit 1
fi

if [ "$DRY_RUN" = "1" ]; then
  echo ">>> DRY_RUN 模式，不写文件。通过的 ${#passed_cidrs[@]} 个段："
  printf '%s\n' "${passed_cidrs[@]}"
  exit 0
fi

printf '%s\n' "${passed_cidrs[@]}" > "$OUTPUT"
echo ">>> 已写入 ${#passed_cidrs[@]} 个段到 $OUTPUT"
