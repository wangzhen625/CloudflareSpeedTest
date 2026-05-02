package task

import (
	"bufio"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/XIU2/CloudflareSpeedTest/utils"
)

const defaultInputFile = "ip.txt"

var (
	// TestAll test all ip
	TestAll = false
	// IPFile is the filename of IP Rangs
	IPFile = defaultInputFile
	IPText string
)

func InitRandSeed() {
	rand.Seed(time.Now().UnixNano())
}

func isIPv4(ip string) bool {
	return strings.Contains(ip, ".")
}

func randIPEndWith(num byte) byte {
	if num == 0 { // 对于 /32 这种单独的 IP
		return byte(0)
	}
	return byte(rand.Intn(int(num)))
}

type IPRanges struct {
	ips     []*net.IPAddr
	mask    string
	firstIP net.IP
	ipNet   *net.IPNet
}

func newIPRanges() *IPRanges {
	return &IPRanges{
		ips: make([]*net.IPAddr, 0),
	}
}

// 如果是单独 IP 则加上子网掩码，反之则获取子网掩码(r.mask)
func (r *IPRanges) fixIP(ip string) string {
	// 如果不含有 '/' 则代表不是 IP 段，而是一个单独的 IP，因此需要加上 /32 /128 子网掩码
	if i := strings.IndexByte(ip, '/'); i < 0 {
		if isIPv4(ip) {
			r.mask = "/32"
		} else {
			r.mask = "/128"
		}
		ip += r.mask
	} else {
		r.mask = ip[i:]
	}
	return ip
}

// 解析 IP 段，获得 IP、IP 范围、子网掩码
func (r *IPRanges) parseCIDR(ip string) {
	var err error
	if r.firstIP, r.ipNet, err = net.ParseCIDR(r.fixIP(ip)); err != nil {
		log.Fatalln("ParseCIDR err", err)
	}
}

func (r *IPRanges) appendIPv4(d byte) {
	r.appendIP(net.IPv4(r.firstIP[12], r.firstIP[13], r.firstIP[14], d))
}

func (r *IPRanges) appendIP(ip net.IP) {
	r.ips = append(r.ips, &net.IPAddr{IP: ip})
}

// 返回第四段 ip 的最小值及可用数目
func (r *IPRanges) getIPRange() (minIP, hosts byte) {
	minIP = r.firstIP[15] & r.ipNet.Mask[3] // IP 第四段最小值

	// 根据子网掩码获取主机数量
	m := net.IPv4Mask(255, 255, 255, 255)
	for i, v := range r.ipNet.Mask {
		m[i] ^= v
	}
	total, _ := strconv.ParseInt(m.String(), 16, 32) // 总可用 IP 数
	if total > 255 {                                 // 矫正 第四段 可用 IP 数
		hosts = 255
		return
	}
	hosts = byte(total)
	return
}

// 非 -allip 模式下，每个 /24 子网随机抽样的 IP 数量。
// 提高这个值能更精确地代表该 /24 段的延迟/速度，但会成倍增加测速时长。
const sampleCountPerSubnet = 3

func (r *IPRanges) chooseIPv4() {
	if r.mask == "/32" { // 单个 IP 则无需随机，直接加入自身即可
		r.appendIP(r.firstIP)
	} else {
		minIP, hosts := r.getIPRange()    // 返回第四段 IP 的最小值及可用数目
		for r.ipNet.Contains(r.firstIP) { // 只要该 IP 没有超出 IP 网段范围，就继续循环随机
			if TestAll { // 如果是测速全部 IP
				for i := 0; i <= int(hosts); i++ { // 遍历 IP 最后一段最小值到最大值
					r.appendIPv4(byte(i) + minIP)
				}
			} else { // 当前 /24 随机抽 N 个互不相同的尾段 IP
				n := sampleCountPerSubnet
				if n > int(hosts) { // 子网过小（如 /30）时降级为该段全部可选值
					n = int(hosts)
				}
				seen := make(map[byte]bool, n)
				for len(seen) < n {
					v := randIPEndWith(hosts)
					if seen[v] {
						continue
					}
					seen[v] = true
					r.appendIPv4(minIP + v)
				}
			}
			r.firstIP[14]++ // 0.0.(X+1).X
			if r.firstIP[14] == 0 {
				r.firstIP[13]++ // 0.(X+1).X.X
				if r.firstIP[13] == 0 {
					r.firstIP[12]++ // (X+1).X.X.X
				}
			}
		}
	}
}

func (r *IPRanges) chooseIPv6() {
	if r.mask == "/128" { // 单个 IP 则无需随机，直接加入自身即可
		r.appendIP(r.firstIP)
	} else {
		var tempIP uint8                  // 临时变量，用于记录前一位的值
		for r.ipNet.Contains(r.firstIP) { // 只要该 IP 没有超出 IP 网段范围，就继续循环随机
			r.firstIP[15] = randIPEndWith(255) // 随机 IP 的最后一段
			r.firstIP[14] = randIPEndWith(255) // 随机 IP 的最后一段

			targetIP := make([]byte, len(r.firstIP))
			copy(targetIP, r.firstIP)
			r.appendIP(targetIP) // 加入 IP 地址池

			for i := 13; i >= 0; i-- { // 从倒数第三位开始往前随机
				tempIP = r.firstIP[i]              // 保存前一位的值
				r.firstIP[i] += randIPEndWith(255) // 随机 0~255，加到当前位上
				if r.firstIP[i] >= tempIP {        // 如果当前位的值大于等于前一位的值，说明随机成功了，可以退出该循环
					break
				}
			}
		}
	}
}

// loadHistoricalIPs 从已有的输出文件中读取上次测速保留的 IP，
// 用于跨次运行保留优质 IP——即便随机抽样这次没抽中，也能再测一次。
// 文件不存在 / 不可读 / 解析为空时静默返回 nil，调用方继续走原有抽样逻辑。
func loadHistoricalIPs() []*net.IPAddr {
	path := utils.Output
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var ips []*net.IPAddr
	scanner := bufio.NewScanner(f)
	headerSkipped := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !headerSkipped {
			headerSkipped = true
			continue
		}
		ipStr := line
		if i := strings.IndexByte(line, ','); i > 0 {
			ipStr = line[:i]
		}
		if ip := net.ParseIP(ipStr); ip != nil {
			ips = append(ips, &net.IPAddr{IP: ip})
		}
	}
	return ips
}

func loadIPRanges() []*net.IPAddr {
	historical := loadHistoricalIPs()

	ranges := newIPRanges()
	if IPText != "" { // 从参数中获取 IP 段数据
		IPs := strings.Split(IPText, ",") // 以逗号分隔为数组并循环遍历
		for _, IP := range IPs {
			IP = strings.TrimSpace(IP) // 去除首尾的空白字符（空格、制表符、换行符等）
			if IP == "" {              // 跳过空的（即开头、结尾或连续多个 ,, 的情况）
				continue
			}
			ranges.parseCIDR(IP) // 解析 IP 段，获得 IP、IP 范围、子网掩码
			if isIPv4(IP) {      // 生成要测速的所有 IPv4 / IPv6 地址（单个/随机/全部）
				ranges.chooseIPv4()
			} else {
				ranges.chooseIPv6()
			}
		}
	} else { // 从文件中获取 IP 段数据
		if IPFile == "" {
			IPFile = defaultInputFile
		}
		file, err := os.Open(IPFile)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() { // 循环遍历文件每一行
			line := strings.TrimSpace(scanner.Text()) // 去除首尾的空白字符（空格、制表符、换行符等）
			if line == "" {                           // 跳过空行
				continue
			}
			ranges.parseCIDR(line) // 解析 IP 段，获得 IP、IP 范围、子网掩码
			if isIPv4(line) {      // 生成要测速的所有 IPv4 / IPv6 地址（单个/随机/全部）
				ranges.chooseIPv4()
			} else {
				ranges.chooseIPv6()
			}
		}
	}

	// 把上次保留的优质 IP 与本次抽样合并并去重（历史 IP 排在前面）
	if len(historical) == 0 {
		return ranges.ips
	}
	seen := make(map[string]struct{}, len(historical)+len(ranges.ips))
	merged := make([]*net.IPAddr, 0, len(historical)+len(ranges.ips))
	for _, ip := range historical {
		key := ip.IP.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, ip)
	}
	for _, ip := range ranges.ips {
		key := ip.IP.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, ip)
	}
	utils.Yellow.Printf("[信息] 从 %s 复用上次保留的 %d 个 IP，合并后共 %d 个待测。\n",
		path(utils.Output), len(historical), len(merged))
	return merged
}

// path 仅用于在日志里展示输出文件路径。
func path(s string) string {
	if s == "" {
		return "result.csv"
	}
	return s
}
