#!/usr/bin/env bash
#
# probe.sh — 验证 Cloudflare IP 在你网络下是否可用（支持串行 / 并发）
#
# 通过 curl --resolve 强制把 speed.cloudflare.com 指向目标 IP，
# 量出 TCP/TLS/总耗时，判断该 IP 是否被 TLS 干扰。
#
# 用法:
#   ./probe.sh                          # 测内置一组代表 IP（串行）
#   ./probe.sh 1.2.3.4 5.6.7.8          # 测命令行传入的 IP
#   ./probe.sh ips.txt                  # 单文件参数（每行一个 IP）
#   ./probe.sh < ips.txt                # 从 stdin 读
#   echo 1.1.1.1 | ./probe.sh           # 同上
#   PARALLEL=10 ./probe.sh < ips.txt    # 并发模式（输出顺序不保证）
#
# 环境变量:
#   PARALLEL=1                          # 并发数；1=串行（默认），>1 走 xargs -P
#   TIMEOUT=4                           # curl -m 超时秒数
#
# 结果判读:
#   tls:0     code:000     → TLS 被干扰，该 IP/段废了
#   tls:<2s   code:200/403 → 正常，握手成功（403 仅是路径错）
#   tcp:0     全为 0       → IP 路由不可达
#

set -u

URL='https://speed.cloudflare.com/__down?bytes=10000'
HOST='speed.cloudflare.com'
PARALLEL="${PARALLEL:-1}"
TIMEOUT="${TIMEOUT:-4}"

probe() {
  local ip="$1"
  [ -z "$ip" ] && return 0
  # 把 IP 和 curl 输出合成一行后一次性 printf，避免并发模式下两次写入交错
  local result
  result=$(curl -w "tcp:%{time_connect}s tls:%{time_appconnect}s total:%{time_total}s code:%{http_code}" \
    -o /dev/null -s -m "$TIMEOUT" -I \
    --resolve "${HOST}:443:${ip}" "$URL")
  printf "%-18s %s\n" "$ip" "$result"
}
export -f probe
export URL HOST TIMEOUT

# 收集要测试的 IP 列表
ips=()
if [ "$#" -eq 1 ] && [ -f "$1" ]; then
  while IFS= read -r ip; do
    ip="${ip%$'\r'}"
    [ -n "$ip" ] && ips+=("$ip")
  done < "$1"
elif [ "$#" -gt 0 ]; then
  ips=("$@")
elif [ ! -t 0 ]; then
  while IFS= read -r ip; do
    ip="${ip%$'\r'}"
    [ -n "$ip" ] && ips+=("$ip")
  done
fi

# 兜底：交互式无参 / 非交互式空 stdin → 测内置代表 IP
if [ "${#ips[@]}" -eq 0 ]; then
  ips=(104.16.0.1 172.64.0.1 172.67.0.1 162.158.0.1 \
       108.162.192.1 141.101.64.1 173.245.48.1 \
       198.41.128.1 131.0.72.1)
fi

if [ "$PARALLEL" -le 1 ]; then
  for ip in "${ips[@]}"; do probe "$ip"; done
else
  printf '%s\n' "${ips[@]}" | xargs -P "$PARALLEL" -I {} bash -c 'probe "$@"' _ {}
fi
