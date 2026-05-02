package utils

import (
	"encoding/csv"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultOutput         = "result.csv"
	maxDelay              = 9999 * time.Millisecond
	minDelay              = 0 * time.Millisecond
	maxLossRate   float32 = 1.0
)

var (
	InputMaxDelay    = maxDelay
	InputMinDelay    = minDelay
	InputMaxLossRate = maxLossRate
	Output           = defaultOutput
	PrintNum         = 10
	Debug            = false // 是否开启调试模式
	TCPPort          = 443   // 由 main.go 从 task.TCPPort 同步过来，用于在 CSV/输出中展示
)

// ColoMap：Cloudflare PoP 机场三字码 → 中文城市名
// 未命中的码会原样输出（仅显示三字码，不带斜杠）
var ColoMap = map[string]string{
	// 中国大陆 / 港澳台
	"HKG": "香港", "TPE": "台北", "KHH": "高雄",
	"PVG": "上海", "SHA": "上海", "PEK": "北京", "CAN": "广州", "SZX": "深圳",
	"CTU": "成都", "XIY": "西安", "WUH": "武汉", "FOC": "福州",
	// 日本
	"NRT": "东京（成田）", "HND": "东京（羽田）", "KIX": "大阪", "ITM": "大阪（伊丹）",
	"FUK": "福冈", "OKA": "冲绳", "NGO": "名古屋",
	// 韩国
	"ICN": "首尔（仁川）", "GMP": "首尔（金浦）", "PUS": "釜山",
	// 东南亚
	"SIN": "新加坡", "BKK": "曼谷", "KUL": "吉隆坡", "MNL": "马尼拉", "CGK": "雅加达",
	"HAN": "河内", "SGN": "胡志明", "PNH": "金边", "RGN": "仰光", "VTE": "万象",
	// 南亚
	"BOM": "孟买", "DEL": "新德里", "MAA": "金奈", "BLR": "班加罗尔", "CCU": "加尔各答",
	"HYD": "海得拉巴", "KHI": "卡拉奇", "ISB": "伊斯兰堡", "DAC": "达卡", "CMB": "科伦坡",
	"KTM": "加德满都",
	// 中东
	"DXB": "迪拜", "AUH": "阿布扎比", "DOH": "多哈", "RUH": "利雅得", "JED": "吉达",
	"KWI": "科威特城", "BAH": "巴林", "MCT": "马斯喀特", "TLV": "特拉维夫",
	"AMM": "安曼", "BEY": "贝鲁特", "BGW": "巴格达", "THR": "德黑兰", "IKA": "德黑兰",
	// 中亚 / 高加索
	"TAS": "塔什干", "ALA": "阿拉木图", "ASB": "阿什哈巴德",
	"GYD": "巴库", "EVN": "埃里温", "TBS": "第比利斯",
	// 美国（西海岸）
	"LAX": "洛杉矶", "SJC": "圣何塞", "SFO": "旧金山", "SEA": "西雅图", "PDX": "波特兰",
	"SAN": "圣地亚哥", "BUR": "伯班克", "OAK": "奥克兰", "SMF": "萨克拉门托",
	"LAS": "拉斯维加斯", "PHX": "凤凰城", "SLC": "盐湖城", "BOI": "博伊西", "ABQ": "阿尔伯克基",
	"DEN": "丹佛", "HNL": "檀香山", "ANC": "安克雷奇",
	// 美国（中部 / 南部）
	"DFW": "达拉斯", "IAH": "休斯顿", "AUS": "奥斯汀", "SAT": "圣安东尼奥",
	"OKC": "俄克拉荷马城", "MCI": "堪萨斯城", "STL": "圣路易斯", "MEM": "孟菲斯",
	"BNA": "纳什维尔", "MSP": "明尼阿波利斯", "OMA": "奥马哈", "IND": "印第安纳波利斯",
	"DTW": "底特律", "ORD": "芝加哥", "MKE": "密尔沃基", "CMH": "哥伦布", "CLE": "克利夫兰",
	"PIT": "匹兹堡", "CVG": "辛辛那提",
	// 美国（东海岸 / 东南）
	"JFK": "纽约", "EWR": "纽瓦克", "LGA": "纽约（拉瓜迪亚）",
	"BOS": "波士顿", "PHL": "费城", "BWI": "巴尔的摩",
	"IAD": "华盛顿", "DCA": "华盛顿（里根）",
	"RIC": "里士满", "RDU": "罗利", "CLT": "夏洛特", "ATL": "亚特兰大",
	"MIA": "迈阿密", "MCO": "奥兰多", "TPA": "坦帕", "JAX": "杰克逊维尔", "FLL": "劳德代尔堡",
	"BUF": "布法罗", "ROC": "罗切斯特",
	// 加拿大
	"YYZ": "多伦多", "YVR": "温哥华", "YUL": "蒙特利尔", "YOW": "渥太华",
	"YYC": "卡尔加里", "YEG": "埃德蒙顿", "YWG": "温尼伯", "YHZ": "哈利法克斯", "YQM": "蒙克顿",
	// 墨西哥 / 中美 / 加勒比
	"MEX": "墨西哥城", "GDL": "瓜达拉哈拉", "MTY": "蒙特雷", "QRO": "克雷塔罗",
	"SJO": "圣何塞（哥斯达黎加）", "GUA": "危地马拉城", "SDQ": "圣多明各", "PTY": "巴拿马城",
	// 南美
	"GRU": "圣保罗", "GIG": "里约热内卢", "BSB": "巴西利亚", "POA": "阿雷格里港", "FOR": "福塔莱萨",
	"EZE": "布宜诺斯艾利斯", "SCL": "圣地亚哥（智利）", "LIM": "利马",
	"BOG": "波哥大", "UIO": "基多", "CCS": "加拉加斯", "ASU": "亚松森",
	// 欧洲（西欧）
	"LHR": "伦敦", "MAN": "曼彻斯特", "EDI": "爱丁堡", "DUB": "都柏林",
	"CDG": "巴黎", "MRS": "马赛",
	"AMS": "阿姆斯特丹", "BRU": "布鲁塞尔", "LUX": "卢森堡",
	"FRA": "法兰克福", "MUC": "慕尼黑", "DUS": "杜塞尔多夫", "HAM": "汉堡",
	"TXL": "柏林", "BER": "柏林",
	"ZRH": "苏黎世", "GVA": "日内瓦", "VIE": "维也纳",
	"MAD": "马德里", "BCN": "巴塞罗那", "LIS": "里斯本",
	"MXP": "米兰", "FCO": "罗马", "PMO": "巴勒莫",
	// 欧洲（北欧）
	"CPH": "哥本哈根", "ARN": "斯德哥尔摩", "OSL": "奥斯陆", "HEL": "赫尔辛基",
	"KEF": "雷克雅未克",
	// 欧洲（东欧 / 中欧 / 巴尔干）
	"WAW": "华沙", "PRG": "布拉格", "BUD": "布达佩斯", "OTP": "布加勒斯特",
	"SOF": "索菲亚", "BEG": "贝尔格莱德", "ZAG": "萨格勒布",
	"KBP": "基辅", "DME": "莫斯科", "SVO": "莫斯科", "VKO": "莫斯科", "LED": "圣彼得堡",
	"RIX": "里加", "VNO": "维尔纽斯", "TLL": "塔林", "MSQ": "明斯克", "KIV": "基希讷乌",
	"ATH": "雅典", "IST": "伊斯坦布尔", "ESB": "安卡拉",
	// 大洋洲
	"SYD": "悉尼", "MEL": "墨尔本", "BNE": "布里斯班", "PER": "珀斯", "ADL": "阿德莱德",
	"CBR": "堪培拉", "HBA": "霍巴特",
	"AKL": "奥克兰（新西兰）", "WLG": "惠灵顿", "CHC": "基督城",
	"NOU": "努美阿", "PPT": "帕皮提", "NAN": "楠迪",
	// 非洲
	"JNB": "约翰内斯堡", "CPT": "开普敦", "DUR": "德班",
	"LOS": "拉各斯", "ABV": "阿布贾", "ACC": "阿克拉", "ABJ": "阿比让",
	"CAI": "开罗", "CMN": "卡萨布兰卡", "TUN": "突尼斯", "ALG": "阿尔及尔",
	"NBO": "内罗毕", "ADD": "亚的斯亚贝巴", "DAR": "达累斯萨拉姆",
	"KGL": "基加利", "EBB": "坎帕拉",
	"MPM": "马普托", "MRU": "毛里求斯", "TNR": "塔那那利佛",
	"DKR": "达喀尔", "OUA": "瓦加杜古", "BKO": "巴马科", "LAD": "罗安达",
}

// 是否打印测试结果
func NoPrintResult() bool {
	return PrintNum == 0
}

// 是否输出到文件
func noOutput() bool {
	return Output == "" || Output == " "
}

type PingData struct {
	IP       *net.IPAddr
	Sended   int
	Received int
	Delay    time.Duration
	Colo     string
}

type CloudflareIPData struct {
	*PingData
	lossRate      float32
	DownloadSpeed float64
}

// 计算丢包率
func (cf *CloudflareIPData) getLossRate() float32 {
	if cf.lossRate == 0 {
		pingLost := cf.Sended - cf.Received
		cf.lossRate = float32(pingLost) / float32(cf.Sended)
	}
	return cf.lossRate
}

func (cf *CloudflareIPData) toString() []string {
	result := make([]string, 9)
	result[0] = cf.IP.String()
	result[1] = strconv.Itoa(TCPPort)
	result[2] = strconv.Itoa(cf.Sended)
	result[3] = strconv.Itoa(cf.Received)
	result[4] = strconv.FormatFloat(float64(cf.getLossRate()), 'f', 2, 32)
	result[5] = strconv.FormatFloat(cf.Delay.Seconds()*1000, 'f', 2, 32)
	result[6] = strconv.FormatFloat(cf.DownloadSpeed/1024/1024, 'f', 2, 32)
	// 数据中心/地区码：空 → N/A；命中 ColoMap → "LAX/洛杉矶"；未命中 → 仅原码
	switch {
	case cf.Colo == "":
		result[7] = "N/A"
	case ColoMap[cf.Colo] != "":
		result[7] = cf.Colo + "/" + ColoMap[cf.Colo]
	default:
		result[7] = cf.Colo
	}
	// 测试时间：按小时截断的本地时间，例 20260502-08:00
	result[8] = time.Now().Truncate(time.Hour).Format("20060102-15:04")
	return result
}

func ExportCsv(data []CloudflareIPData) {
	if noOutput() || len(data) == 0 {
		return
	}
	fp, err := os.Create(Output)
	if err != nil {
		log.Fatalf("创建文件[%s]失败：%v", Output, err)
		return
	}
	defer fp.Close()
	w := csv.NewWriter(fp) //创建一个新的写入文件流
	_ = w.Write(csvHeader())
	_ = w.WriteAll(convertToString(data))
	w.Flush()
}

func csvHeader() []string {
	return []string{"IP地址", "端口", "已发送", "已接收", "丢包率", "平均延迟", "下载速度(MB/s)", "数据中心", "测试时间"}
}

// IncrementalCsvWriter 在下载测速过程中逐条写入结果。
// 直接写 *os.File 并在每行后 Sync，绕过 csv.Writer 内部 bufio 缓冲，
// 即便在 SIGINT/SIGKILL 等不会执行 defer 的中断场景下，
// 已测过的 IP 数据也已 fsync 到磁盘。
type IncrementalCsvWriter struct {
	fp *os.File
}

// NewIncrementalCsvWriter 创建并打开输出文件，写入表头。
// 当未配置输出文件 (Output 为空) 时返回 (nil, nil)；
// 调用方可对 nil 安全调用 Write/Close。
func NewIncrementalCsvWriter() (*IncrementalCsvWriter, error) {
	if noOutput() {
		return nil, nil
	}
	fp, err := os.Create(Output)
	if err != nil {
		return nil, fmt.Errorf("创建文件[%s]失败: %w", Output, err)
	}
	if _, err := fp.WriteString(joinCsvLine(csvHeader())); err != nil {
		fp.Close()
		return nil, fmt.Errorf("写入表头失败: %w", err)
	}
	if err := fp.Sync(); err != nil {
		fp.Close()
		return nil, fmt.Errorf("刷新表头失败: %w", err)
	}
	return &IncrementalCsvWriter{fp: fp}, nil
}

// Write 写入一行测速结果并 Sync 强制落盘。
func (iw *IncrementalCsvWriter) Write(d CloudflareIPData) {
	if iw == nil {
		return
	}
	if _, err := iw.fp.WriteString(joinCsvLine(d.toString())); err != nil {
		return
	}
	_ = iw.fp.Sync()
}

func (iw *IncrementalCsvWriter) Close() {
	if iw == nil {
		return
	}
	_ = iw.fp.Sync()
	_ = iw.fp.Close()
}

// joinCsvLine 把单行字段拼成 CSV，遇到含逗号/引号/换行的字段自动加引号转义。
func joinCsvLine(fields []string) string {
	var b strings.Builder
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		if strings.ContainsAny(f, ",\"\n\r") {
			b.WriteByte('"')
			b.WriteString(strings.ReplaceAll(f, `"`, `""`))
			b.WriteByte('"')
		} else {
			b.WriteString(f)
		}
	}
	b.WriteByte('\n')
	return b.String()
}

func convertToString(data []CloudflareIPData) [][]string {
	result := make([][]string, 0)
	for _, v := range data {
		result = append(result, v.toString())
	}
	return result
}

// 延迟丢包排序
type PingDelaySet []CloudflareIPData

// 延迟条件过滤
func (s PingDelaySet) FilterDelay() (data PingDelaySet) {
	if InputMaxDelay > maxDelay || InputMinDelay < minDelay { // 当输入的延迟条件不在默认范围内时，不进行过滤
		return s
	}
	if InputMaxDelay == maxDelay && InputMinDelay == minDelay { // 当输入的延迟条件为默认值时，不进行过滤
		return s
	}
	for _, v := range s {
		if v.Delay > InputMaxDelay { // 平均延迟上限，延迟大于条件最大值时，后面的数据都不满足条件，直接跳出循环
			break
		}
		if v.Delay < InputMinDelay { // 平均延迟下限，延迟小于条件最小值时，不满足条件，跳过
			continue
		}
		data = append(data, v) // 延迟满足条件时，添加到新数组中
	}
	return
}

// 丢包条件过滤
func (s PingDelaySet) FilterLossRate() (data PingDelaySet) {
	if InputMaxLossRate >= maxLossRate { // 当输入的丢包条件为默认值时，不进行过滤
		return s
	}
	for _, v := range s {
		if v.getLossRate() > InputMaxLossRate { // 丢包几率上限
			break
		}
		data = append(data, v) // 丢包率满足条件时，添加到新数组中
	}
	return
}

func (s PingDelaySet) Len() int {
	return len(s)
}
func (s PingDelaySet) Less(i, j int) bool {
	iRate, jRate := s[i].getLossRate(), s[j].getLossRate()
	if iRate != jRate {
		return iRate < jRate
	}
	return s[i].Delay < s[j].Delay
}
func (s PingDelaySet) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// 下载速度排序
type DownloadSpeedSet []CloudflareIPData

func (s DownloadSpeedSet) Len() int {
	return len(s)
}
func (s DownloadSpeedSet) Less(i, j int) bool {
	return s[i].DownloadSpeed > s[j].DownloadSpeed
}
func (s DownloadSpeedSet) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s DownloadSpeedSet) Print() {
	if NoPrintResult() {
		return
	}
	if len(s) <= 0 { // IP数组长度(IP数量) 大于 0 时继续
		fmt.Println("\n[信息] 完整测速结果 IP 数量为 0，跳过输出结果。")
		return
	}
	dateString := convertToString(s) // 转为多维数组 [][]String
	if len(dateString) < PrintNum {  // 如果IP数组长度(IP数量) 小于  打印次数，则次数改为IP数量
		PrintNum = len(dateString)
	}
	headFormat := "%-16s%-6s%-5s%-5s%-5s%-6s%-12s%-22s\n"
	dataFormat := "%-18s%-8s%-8s%-8s%-8s%-10s%-16s%-22s\n"
	for i := 0; i < PrintNum; i++ { // 如果要输出的 IP 中包含 IPv6，那么就需要调整一下间隔
		if len(dateString[i][0]) > 15 {
			headFormat = "%-40s%-6s%-5s%-5s%-5s%-6s%-12s%-22s\n"
			dataFormat = "%-42s%-8s%-8s%-8s%-8s%-10s%-16s%-22s\n"
			break
		}
	}
	Cyan.Printf(headFormat, "IP地址", "端口", "已发送", "已接收", "丢包率", "平均延迟", "下载速度(MB/s)", "数据中心")
	for i := 0; i < PrintNum; i++ {
		fmt.Printf(dataFormat, dateString[i][0], dateString[i][1], dateString[i][2], dateString[i][3], dateString[i][4], dateString[i][5], dateString[i][6], dateString[i][7])
	}
	if !noOutput() {
		fmt.Printf("\n完整测速结果已写入 %v 文件，可使用记事本/表格软件查看。\n", Output)
	}
}
