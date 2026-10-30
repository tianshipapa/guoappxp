package core

// 红果新（sourceHongguoNew）取流链路。
//
// 移植自 hongguo-android 的 AppSource / PlaybackDevice（GPL-3.0-only，
// 上游同步自 jackotom/hongguo-drama、tsukiyo-zero/hongguoTV、sunnysky123/hongguo-web）。
//
// 与既有「红果」源并存、互不影响：
//   - 目录 / 详情 / 搜索仍复用红果原有的签名链路（provider_hongguo_*.go），
//     这些接口不受 2026-10-27 上游变化影响；
//   - 只有「取播放地址」这一段换成本文件的实现：
//     1. 用 log5-applog-lf.fqnovel.com 的 device_register 注册匿名播放身份（ttEncrypt 加密请求体）；
//     2. POST multi_video_model/v1，带注册身份、不带任何签名头；
//     3. 响应可能是内联 video_list，也可能只给 fallback_api 需要二次 GET；
//     4. 媒体地址可能被 key_seed 二次加密（AES-128-CBC），需先解密；
//     5. spade_a -> content key 的推导与既有 hongguoContentKey 等价，直接复用。
//
// 之所以单独做一个源而不是替换红果：老 video_model/v1 签名链路对部分分集仍可用，
// 两条链路各自兜底，用户在界面上可以按集切换源。

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// hongguoxinVideoAPI 播放接口（尾斜杠照抄 App 实现）。
	hongguoxinVideoAPI = "https://api5-normal-sinfonlineb.fqnovel.com/novel/player/multi_video_model/v1/"

	// hongguoxinVideoUA 播放接口 UA（与「注册身份 + 无签名」路径配套）。
	hongguoxinVideoUA = "com.phoenix.read/71532 (Linux; U; Android 9; SM-N9860; " +
		"Build/PQ3A.190705.10241111;tt-ok/3.12.13.20)"

	// hongguoxinRegisterURL 匿名播放设备注册地址。
	hongguoxinRegisterURL = "https://log5-applog-lf.fqnovel.com/service/2/device_register/" +
		"?tt_data=a&cronet_version=8d40f833_2026-03-03" +
		"&ttnet_version=4.2.243.31-douyin&use_store_region_cookie=1"

	// hongguoxinRegisterUA 注册请求 UA。
	hongguoxinRegisterUA = "com.phoenix.read/72232 (Linux; U; Android 16; zh_CN; 25053RT47C; " +
		"Build/BP2A.250605.031.A3)"

	// hongguoxinVersionName 播放请求里声明的 App 版本。
	hongguoxinVersionName = "7.1.3.32"

	// hongguoxinRegisterCooldown 注册失败后复用失败结果的时长，避免预加载与重试造成注册风暴。
	hongguoxinRegisterCooldown = 30 * time.Second
)

// hongguoxinSalt ttEncrypt 与媒体 URL 解密共用的 64 字节常量段。
var hongguoxinSalt = mustHex(
	"4dd4c2e6b83162090e52b3c7a6733ba41cb2462b829ab58a196b39db57177524" +
		"f49baf7f08e8d68d26a72e37c1a95a2f1f05a51892aef2949732b62a38aadd58")

// hongguoxinMagic ttEncrypt 报文头。
var hongguoxinMagic = mustHex("746305100000")

// hongguoxinAPIDomains fallback_api 允许的域名（防签名 body 被带往第三方）。
var hongguoxinAPIDomains = []string{"fqnovel.com", "snssdk.com"}

// hongguoxinVODDomains 媒体 CDN 允许的域名。
var hongguoxinVODDomains = []string{"qznovelvod.com", "fqnovelvod.com", "douyinvod.com"}

// errHongguixinEmptyPlayback 播放接口返回空响应，通常代表当前匿名身份被软拦截。
var errHongguixinEmptyPlayback = errors.New("红果新接口返回空响应，疑似播放设备身份被拒")

// mustHex 解码写死的十六进制常量。
func mustHex(value string) []byte {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		panic(err)
	}
	return decoded
}

// hongguoxinDevice 注册得到的匿名播放身份。
type hongguoxinDevice struct {
	DeviceID string
	IID      string
	CDID     string
}

// hongguoxinClient 进程内共享的播放身份；红果新与其目录请求（复用红果身份）互不干扰。
type hongguoxinClient struct {
	mu         sync.Mutex
	device     hongguoxinDevice
	lastErr    error
	lastErrAt  time.Time
	registerMu sync.Mutex
}

func (downloader *Downloader) hongguoxinClient() *hongguoxinClient {
	downloader.hongguoxinOnce.Do(func() {
		downloader.hongguoxin = &hongguoxinClient{}
	})
	return downloader.hongguoxin
}

// identity 取进程内共享的匿名播放身份，首次调用时注册。
func (client *hongguoxinClient) identity(ctx context.Context, downloader *Downloader) (hongguoxinDevice, error) {
	client.mu.Lock()
	if client.device.DeviceID != "" {
		device := client.device
		client.mu.Unlock()
		return device, nil
	}
	if client.lastErr != nil && time.Since(client.lastErrAt) < hongguoxinRegisterCooldown {
		err := client.lastErr
		client.mu.Unlock()
		return hongguoxinDevice{}, err
	}
	client.mu.Unlock()
	return client.register(ctx, downloader, "")
}

// renew 在播放接口拒绝了 rejected 身份时换一个新身份；另一处已换过则直接复用。
func (client *hongguoxinClient) renew(ctx context.Context, downloader *Downloader, rejected hongguoxinDevice) (hongguoxinDevice, error) {
	client.mu.Lock()
	if client.device.DeviceID != "" && client.device != rejected {
		device := client.device
		client.mu.Unlock()
		return device, nil
	}
	client.mu.Unlock()
	return client.register(ctx, downloader, rejected.DeviceID)
}

// register 串行化注册，让并发的播放与预加载等待同一次注册。
func (client *hongguoxinClient) register(ctx context.Context, downloader *Downloader, rejected string) (hongguoxinDevice, error) {
	client.registerMu.Lock()
	defer client.registerMu.Unlock()
	client.mu.Lock()
	if client.device.DeviceID != "" && client.device.DeviceID != rejected {
		device := client.device
		client.mu.Unlock()
		return device, nil
	}
	client.mu.Unlock()
	device, err := downloader.registerHongguoxinDevice(ctx)
	client.mu.Lock()
	defer client.mu.Unlock()
	if err != nil {
		if ctx.Err() == nil {
			client.lastErr, client.lastErrAt = err, time.Now()
		}
		return hongguoxinDevice{}, err
	}
	client.device, client.lastErr = device, nil
	return device, nil
}

// registerHongguoxinDevice 向 device_register 注册匿名播放身份。
// 上游自 2026-10-27 起校验设备身份，随机合成的 iid/device_id 会被软拦截（HTTP 200 + 0 字节）。
func (downloader *Downloader) registerHongguoxinDevice(ctx context.Context) (hongguoxinDevice, error) {
	now := time.Now()
	cdid := uuidLike()
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return hongguoxinDevice{}, err
	}
	payload, err := json.Marshal(hongguoxinRegisterPayload(now.UnixMilli(), cdid))
	if err != nil {
		return hongguoxinDevice{}, err
	}
	body, err := hongguoxinEncryptPayload(payload, random)
	if err != nil {
		return hongguoxinDevice{}, err
	}
	stub := md5.Sum(body)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, hongguoxinRegisterURL, bytes.NewReader(body))
	if err != nil {
		return hongguoxinDevice{}, err
	}
	request.Header.Set("User-Agent", hongguoxinRegisterUA)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/octet-stream;tt-data=a")
	request.Header.Set("log-encode-type", "gzip")
	request.Header.Set("x-ss-dp", "8662")
	request.Header.Set("x-ss-req-ticket", strconv.FormatInt(now.UnixMilli(), 10))
	request.Header.Set("x-vc-bdturing-sdk-version", "4.0.3.cn")
	request.Header.Set("lc", "101")
	request.Header.Set("sdk-version", "2")
	request.Header.Set("passport-sdk-version", "5051452")
	request.Header.Set("x-neptune", "-8|50:51:59:30:40:47:49:39")
	request.Header.Set("x-ss-stub", strings.ToUpper(hex.EncodeToString(stub[:])))
	response, err := downloader.doPreparedCatalogRequest(request, 8*time.Second, nil)
	if err != nil {
		return hongguoxinDevice{}, err
	}
	content, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	response.Body.Close()
	if readErr != nil {
		return hongguoxinDevice{}, readErr
	}
	if response.StatusCode != http.StatusOK {
		return hongguoxinDevice{}, fmt.Errorf("红果新设备注册 HTTP %d", response.StatusCode)
	}
	var result map[string]any
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		return hongguoxinDevice{}, errors.New("红果新设备注册返回格式异常")
	}
	code := firstNonEmpty(mapString(result, "code", "Code"), "0")
	if code != "0" {
		return hongguoxinDevice{}, fmt.Errorf("红果新设备注册被拒绝（%s）", truncate(code, 20))
	}
	deviceID := hongguoxinResponseID(result, "device_id")
	iid := hongguoxinResponseID(result, "install_id")
	if !hongguoNumericID.MatchString(deviceID) || !hongguoNumericID.MatchString(iid) || deviceID == "0" || iid == "0" {
		return hongguoxinDevice{}, errors.New("红果新设备注册未返回设备号")
	}
	return hongguoxinDevice{DeviceID: deviceID, IID: iid, CDID: cdid}, nil
}

// hongguoxinResponseID 优先取 <name>_str，其次取数值字段；零值与非法值一律视为缺失。
func hongguoxinResponseID(result map[string]any, name string) string {
	value := mapString(result, name+"_str")
	if value == "" {
		value = mapString(result, name)
	}
	value = strings.TrimSpace(value)
	if !hongguoNumericID.MatchString(value) || value == "0" {
		return ""
	}
	return value
}

// hongguoxinRegisterPayload 注册 JSON：固定匿名安卓机型参数，不含任何账号或真实设备信息。
func hongguoxinRegisterPayload(nowMillis int64, cdid string) map[string]any {
	custom := map[string]any{
		"account_region_src": "fallback",
		"account_region":     "cn",
		"host_bit":           64,
		"dragon_device_type": "phone",
	}
	header := map[string]any{
		"display_name":           "红果免费短剧",
		"update_version_code":    73932,
		"manifest_version_code":  73932,
		"app_version_minor":      "7.3.9.32",
		"aid":                    8662,
		"channel":                "update_64",
		"package":                "com.phoenix.read",
		"app_version":            "7.3.9.32",
		"version_code":           73932,
		"sdk_version":            "3.7.0-rc.34-fanqie-xiaoshuo",
		"sdk_target_version":     29,
		"git_hash":               "3a88f4e",
		"os":                     "Android",
		"os_version":             "16",
		"os_api":                 36,
		"device_model":           "25053RT47C",
		"device_brand":           "Redmi",
		"device_manufacturer":    "Redmi",
		"cpu_abi":                "arm64-v8a",
		"release_build":          "b5e187f",
		"density_dpi":            520,
		"display_density":        "mdpi",
		"resolution":             "1280x2772",
		"language":               "zh",
		"timezone":               8,
		"access":                 "wifi",
		"not_request_sender":     0,
		"carrier":                "CHINA MOBILE",
		"mcc_mnc":                "46000",
		"rom":                    "eng.root.20231116.160340",
		"rom_version":            "W528JS release-keys",
		"cdid":                   cdid,
		"sig_hash":               "56a962410c494bbaf0b58dba20cae56f",
		"openudid":               randomHex(8),
		"clientudid":             uuidLike(),
		"ipv6_list":              []any{},
		"region":                 "CN",
		"tz_name":                "Asia/Shanghai",
		"tz_offset":              28800,
		"sim_region":             "cn",
		"sim_serial_number":      []any{},
		"oaid_may_support":       false,
		"req_id":                 uuidLike(),
		"device_platform":        "android",
		"custom":                 custom,
		"pre_installed_channel":  "",
		"apk_first_install_time": nowMillis,
		"is_system_app":          0,
		"sdk_flavor":             "china",
		"guest_mode":             0,
	}
	return map[string]any{"magic_tag": "ss_app_log", "header": header, "_gen_time": nowMillis}
}

// hongguoxinEncryptPayload ttEncrypt(gzip(payload))：
// 746305100000(6B) || random(32B) || AES-128-CBC(SHA512(gzip) || gzip)。
func hongguoxinEncryptPayload(payload, random []byte) ([]byte, error) {
	if len(random) != 32 {
		return nil, errors.New("红果新注册随机数长度无效")
	}
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(payload); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	digest := sha512.Sum512(compressed.Bytes())
	plain := make([]byte, 0, len(digest)+compressed.Len())
	plain = append(plain, digest[:]...)
	plain = append(plain, compressed.Bytes()...)
	derived := hongguoxinDeriveKeyIV(random)
	block, err := aes.NewCipher(derived[:16])
	if err != nil {
		return nil, err
	}
	plain = pkcs7Pad(plain, aes.BlockSize)
	encrypted := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, derived[16:32]).CryptBlocks(encrypted, plain)
	body := make([]byte, 0, len(hongguoxinMagic)+len(random)+len(encrypted))
	body = append(body, hongguoxinMagic...)
	body = append(body, random...)
	body = append(body, encrypted...)
	return body, nil
}

// hongguoxinDeriveKeyIV SHA512(SHA512(random) || SALT)，取前 32 字节作为 key/iv。
func hongguoxinDeriveKeyIV(random []byte) []byte {
	inner := sha512.Sum512(random)
	outer := sha512.New()
	outer.Write(inner[:])
	outer.Write(hongguoxinSalt)
	return outer.Sum(nil)[:32]
}

// resolveHongguoxinMedia 取某集的播放地址：multi_video_model/v1 -> 内联 / fallback -> key_seed 解密。
func (d *Downloader) resolveHongguoxinMedia(ctx context.Context, videoID string) (providerMedia, error) {
	if !hongguoNumericID.MatchString(videoID) {
		return providerMedia{}, errors.New("红果新分集编号无效")
	}
	client := d.hongguoxinClient()
	device, err := client.identity(ctx, d)
	if err != nil {
		return providerMedia{}, err
	}
	media, err := d.hongguoxinPlayback(ctx, device, videoID)
	if errors.Is(err, errHongguixinEmptyPlayback) && ctx.Err() == nil {
		renewed, renewErr := client.renew(ctx, d, device)
		if renewErr == nil && renewed.DeviceID != device.DeviceID {
			media, err = d.hongguoxinPlayback(ctx, renewed, videoID)
		}
	}
	return media, err
}

// hongguoxinPlayback 发一次无签名播放请求并解析出可播放轨道。
func (d *Downloader) hongguoxinPlayback(ctx context.Context, device hongguoxinDevice, videoID string) (providerMedia, error) {
	body, err := json.Marshal(hongguoxinPlaybackPayload(videoID))
	if err != nil {
		return providerMedia{}, err
	}
	address := hongguoxinPlaybackURL(device, time.Now().UnixMilli())
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(body))
	if err != nil {
		return providerMedia{}, err
	}
	request.Header.Set("User-Agent", hongguoxinVideoUA)
	request.Header.Set("Accept", "application/json; charset=utf-8,application/x-protobuf")
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	response, err := d.doPreparedCatalogRequest(request, 15*time.Second, nil)
	if err != nil {
		return providerMedia{}, err
	}
	content, readErr := io.ReadAll(io.LimitReader(response.Body, providerMaxBodyBytes+1))
	response.Body.Close()
	if readErr != nil {
		return providerMedia{}, readErr
	}
	if response.StatusCode != http.StatusOK {
		return providerMedia{}, fmt.Errorf("红果新播放接口 HTTP %d", response.StatusCode)
	}
	if len(content) == 0 {
		return providerMedia{}, errHongguixinEmptyPlayback
	}
	if len(content) > providerMaxBodyBytes {
		return providerMedia{}, errors.New("红果新播放接口返回数据过大")
	}
	var result map[string]any
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil || result == nil {
		return providerMedia{}, errors.New("红果新播放信息格式无效")
	}
	if err := hongguoxinCheckCode(result); err != nil {
		return providerMedia{}, err
	}
	data, _ := result["data"].(map[string]any)
	if len(data) == 0 {
		return providerMedia{}, errors.New("红果新接口未返回播放数据")
	}
	entry, _ := data[videoID].(map[string]any)
	if len(entry) == 0 {
		return providerMedia{}, errors.New("红果新接口未返回所选分集")
	}
	model, err := hongguoxinObject(entry["video_model"])
	if err != nil || len(model) == 0 {
		return providerMedia{}, errors.New("红果新接口未返回 video_model")
	}
	return d.hongguoxinSelect(ctx, model)
}

// hongguoxinSelect 解析片源模型：内联 video_list 直接用，否则跟随 fallback_api 二次请求。
func (d *Downloader) hongguoxinSelect(ctx context.Context, model map[string]any) (providerMedia, error) {
	info := model
	inline := len(hongguoxinVideoItems(model)) > 0
	if !inline {
		fallback := hongguoxinFallbackAPI(model["fallback_api"])
		if fallback == "" {
			return providerMedia{}, errors.New("红果新接口未返回本集片源地址")
		}
		checked, err := hongguoxinCheckedURL(fallback, hongguoxinAPIDomains)
		if err != nil {
			return providerMedia{}, err
		}
		outer, err := d.hongguoxinPlainGet(ctx, checked)
		if err != nil {
			return providerMedia{}, err
		}
		if err := hongguoxinCheckCode(outer); err != nil {
			return providerMedia{}, err
		}
		videoInfo, _ := outer["video_info"].(map[string]any)
		if err := hongguoxinCheckCode(videoInfo); err != nil {
			return providerMedia{}, err
		}
		info, _ = videoInfo["data"].(map[string]any)
		if len(info) == 0 {
			return providerMedia{}, errors.New("红果新接口未返回本集片源模型")
		}
	}
	var seed []byte
	if encoded := mapString(info, "key_seed"); encoded != "" {
		decoded, err := decodeHongguoBase64(encoded)
		if err != nil {
			return providerMedia{}, errors.New("红果新片源密钥种子无效")
		}
		seed = decoded
	}
	rows := hongguoxinVideoItems(info)
	if len(rows) == 0 {
		return providerMedia{}, errors.New("红果新本集未返回可用片源列表")
	}
	type scored struct {
		item     map[string]any
		rank     int
		pixels   int
		shortSide int
		quality  string
	}
	var candidates []scored
	for _, row := range rows {
		item, _ := row.(map[string]any)
		if len(item) == 0 {
			continue
		}
		meta := nestedMap(item, "video_meta")
		codec := strings.ToLower(firstNonEmpty(mapString(item, "codec_type"), mapString(meta, "codec_type")))
		rank := hongguoxinCodecRank(codec)
		if rank >= 2 {
			// bytevc2 等编码客户端解不了，直接丢弃。
			continue
		}
		quality := firstNonEmpty(
			mapString(item, "quality_desc"), mapString(item, "definition"),
			mapString(meta, "definition"), mapString(item, "height"),
			mapString(item, "vheight"), mapString(meta, "vheight"),
		)
		width, height := hongguoxinDimensions(item, meta)
		candidates = append(candidates, scored{
			item: item, rank: rank, pixels: hongguoxinPixels(quality),
			shortSide: min(width, height), quality: quality,
		})
	}
	if len(candidates) == 0 {
		return providerMedia{}, errors.New("红果新本集暂未提供兼容的 H.264/HEVC 片源")
	}
	// 自动选流：先 H.264 优先，再清晰度优先。
	sort.SliceStable(candidates, func(left, right int) bool {
		if candidates[left].rank != candidates[right].rank {
			return candidates[left].rank < candidates[right].rank
		}
		return candidates[left].pixels > candidates[right].pixels
	})
	var keyErr error
	var variants []providerMedia
	for _, candidate := range candidates {
		spade := firstNonEmpty(
			mapString(candidate.item, "spade_a"),
			mapString(nestedMap(candidate.item, "encrypt_info"), "spade_a"),
		)
		var key []byte
		if spade == "" {
			// 内联模型声明加密（或未声明）却缺少 spade_a：不能当作明文播放。
			if inline && !hongguoxinDeclaredPlain(candidate.item) {
				continue
			}
		} else {
			var err error
			key, err = hongguoContentKey(spade)
			if err != nil {
				keyErr = err
				continue
			}
		}
		for _, raw := range hongguoxinURLCandidates(candidate.item) {
			address := hongguoxinCleanURL(raw)
			if seed != nil {
				decoded, err := hongguoxinDecryptURL(address, seed)
				if err != nil {
					continue
				}
				address = decoded
			}
			checked, err := hongguoxinCheckedURL(address, hongguoxinVODDomains)
			if err != nil {
				continue
			}
			variants = append(variants, providerMedia{
				URL: checked, Referer: "https://novel.snssdk.com/", CENCKey: key, Quality: candidate.pixels,
			})
		}
	}
	if len(variants) == 0 {
		if keyErr != nil {
			return providerMedia{}, fmt.Errorf("红果新媒体密钥不可用: %w", keyErr)
		}
		return providerMedia{}, errors.New("红果新本集片源地址或密钥无法解析")
	}
	selected := variants[0]
	selected.Variants = variants
	return selected, nil
}

// hongguoxinPlaybackURL 按固定字段序拼播放请求 query。
// 注意 resolution=1280*2772 里的 * 要编成 %2A；Go 的 url.Values.Encode 会按 key 排序，
// 上游对字段序敏感，因此这里手动拼接。
func hongguoxinPlaybackURL(device hongguoxinDevice, rticket int64) string {
	pairs := [][2]string{
		{"ac", "wifi"}, {"channel", "update_64"}, {"aid", "8662"}, {"app_name", "novelread"},
		{"version_code", "71332"}, {"version_name", hongguoxinVersionName},
		{"device_platform", "android"}, {"os", "android"}, {"ssmix", "a"},
		{"device_type", "25053RT47C"}, {"device_brand", "Redmi"}, {"language", "zh"},
		{"os_api", "36"}, {"os_version", "16"}, {"manifest_version_code", "71332"},
		{"resolution", "1280*2772"}, {"dpi", "520"}, {"update_version_code", "71332"},
		{"host_abi", "arm64-v8a"}, {"dragon_device_type", "phone"}, {"pv_player", "71332"},
		{"compliance_status", "0"}, {"need_personal_recommend", "1"},
		{"player_so_load", "1"}, {"is_android_pad_screen", "0"},
		{"device_id", device.DeviceID}, {"iid", device.IID}, {"cdid", device.CDID},
		{"_rticket", strconv.FormatInt(rticket, 10)},
	}
	var builder strings.Builder
	builder.Grow(512)
	for index, pair := range pairs {
		if index > 0 {
			builder.WriteByte('&')
		}
		builder.WriteString(hongguoxinPyEncode(pair[0]))
		builder.WriteByte('=')
		builder.WriteString(hongguoxinPyEncode(pair[1]))
	}
	return hongguoxinVideoAPI + "?" + builder.String()
}

// hongguoxinPyEncode 与 python urllib.parse.quote / App 实现一致的编码：
// 只保留 A-Za-z0-9-_.~，其余（含 !'()* 与 /）一律编成大写 %XX。
func hongguoxinPyEncode(raw string) string {
	var builder strings.Builder
	builder.Grow(len(raw))
	for index := 0; index < len(raw); index++ {
		current := raw[index]
		if current >= 'A' && current <= 'Z' || current >= 'a' && current <= 'z' ||
			current >= '0' && current <= '9' ||
			current == '-' || current == '_' || current == '.' || current == '~' {
			builder.WriteByte(current)
			continue
		}
		builder.WriteString(fmt.Sprintf("%%%02X", current))
	}
	return builder.String()
}

// hongguoxinPlaybackPayload 播放请求体（biz_param 与红果老链路一致，注意不带 dr_scene）。
func hongguoxinPlaybackPayload(videoID string) map[string]any {
	// device_level=1：level=3 会把部分剧集的 360P-720P 下发成 bytevc2（无法解码），只剩 1080P。
	biz := map[string]any{
		"detail_page_version":       0,
		"device_level":              1,
		"disable_digg_stat":         false,
		"need_all_video_definition": true,
		"need_mp4_align":            false,
		"use_os_player":             false,
		"use_server_dns":            false,
		"video_platform":            1024,
	}
	return map[string]any{
		"biz_param":          biz,
		"mixed_video_id_map": map[string]any{"1004": []string{videoID}},
	}
}

// hongguoxinPlainGet fallback_api 的普通 GET：不带签名，只带 UA。
func (d *Downloader) hongguoxinPlainGet(ctx context.Context, address string) (map[string]any, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", hongguoxinVideoUA)
	response, err := d.doPreparedCatalogRequest(request, 15*time.Second, nil)
	if err != nil {
		return nil, err
	}
	content, readErr := io.ReadAll(io.LimitReader(response.Body, providerMaxBodyBytes+1))
	response.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("红果新片源兜底接口 HTTP %d", response.StatusCode)
	}
	var result map[string]any
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil || result == nil {
		return nil, errors.New("红果新播放信息格式无效")
	}
	return result, nil
}

// hongguoxinCheckCode 校验响应里的 code / Code 都为 0（缺失视为 0）。
func hongguoxinCheckCode(result map[string]any) error {
	for _, key := range []string{"code", "Code"} {
		if value := mapString(result, key); value != "" && value != "0" {
			return fmt.Errorf("红果新接口暂未提供本集播放信息（%s）", truncate(value, 20))
		}
	}
	return nil
}

// hongguoxinObject video_model 可能是 JSON 字符串或对象。
func hongguoxinObject(value any) (map[string]any, error) {
	if text, ok := value.(string); ok {
		if text == "" {
			return nil, nil
		}
		if decoded, err := decodeHongguoBase64(text); err == nil && len(decoded) > 0 && decoded[0] == '{' {
			text = string(decoded)
		}
		var model map[string]any
		decoder := json.NewDecoder(strings.NewReader(text))
		decoder.UseNumber()
		if err := decoder.Decode(&model); err != nil {
			return nil, err
		}
		return model, nil
	}
	model, _ := value.(map[string]any)
	return model, nil
}

// hongguoxinFallbackAPI 解析 fallback_api：字符串 / [url] / {"fallback_api": url} 都可能。
func hongguoxinFallbackAPI(value any) string {
	switch item := value.(type) {
	case string:
		trimmed := strings.TrimSpace(item)
		if strings.HasPrefix(trimmed, "{") {
			var parsed map[string]any
			if json.Unmarshal([]byte(trimmed), &parsed) == nil {
				if nested := mapString(parsed, "fallback_api"); nested != "" {
					return nested
				}
			}
		}
		return trimmed
	case []any:
		if len(item) > 0 {
			text, _ := item[0].(string)
			return strings.TrimSpace(text)
		}
	case map[string]any:
		return mapString(item, "fallback_api")
	}
	return ""
}

// hongguoxinVideoItems video_list 可能是数组，也可能是 map（fallback 模型）。
func hongguoxinVideoItems(info map[string]any) []any {
	raw, found := info["video_list"]
	if !found {
		return nil
	}
	if rows, ok := raw.([]any); ok {
		return rows
	}
	if rows, ok := raw.(map[string]any); ok {
		keys := make([]string, 0, len(rows))
		for key := range rows {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make([]any, 0, len(keys))
		for _, key := range keys {
			out = append(out, rows[key])
		}
		return out
	}
	return nil
}

// hongguoxinCodecRank 0=H.264，1=HEVC，2=其它（丢弃）。
func hongguoxinCodecRank(codec string) int {
	switch codec {
	case "h264", "avc", "avc1":
		return 0
	case "hevc", "h265", "hvc1", "hev1", "bytevc1":
		return 1
	}
	return 2
}

func hongguoxinDimensions(item, meta map[string]any) (int, int) {
	pairs := [][4]string{
		{"vwidth", "vheight", "", ""},
		{"width", "height", "", ""},
	}
	for _, keys := range pairs {
		width, _ := strconv.Atoi(firstNonEmpty(mapString(item, keys[0]), mapString(meta, keys[0])))
		height, _ := strconv.Atoi(firstNonEmpty(mapString(item, keys[1]), mapString(meta, keys[1])))
		if width > 0 && height > 0 {
			return width, height
		}
	}
	return 0, 0
}

// hongguoxinPixels 取字符串里第一组数字（如 1080p -> 1080）。
func hongguoxinPixels(value string) int {
	digits := hongguoQualityNumber.FindString(value)
	if digits == "" {
		return 0
	}
	number, _ := strconv.Atoi(digits)
	return number
}

// hongguoxinURLCandidates main_url / backup_url / backup_url_list 的候选序列。
func hongguoxinURLCandidates(item map[string]any) []string {
	var out []string
	add := func(value any) {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			out = append(out, strings.TrimSpace(text))
		}
	}
	add(item["main_url"])
	add(item["backup_url"])
	if rows, ok := item["backup_url_list"].([]any); ok {
		for _, row := range rows {
			add(row)
		}
	}
	return out
}

// hongguoxinDeclaredPlain encrypt_info.encrypt 明确为 false / 0 时才视为明文片源。
func hongguoxinDeclaredPlain(item map[string]any) bool {
	flag, found := nestedMap(item, "encrypt_info")["encrypt"]
	if !found {
		return false
	}
	switch value := flag.(type) {
	case bool:
		return !value
	case string:
		text := strings.ToLower(strings.TrimSpace(value))
		return text == "0" || text == "false"
	case json.Number:
		return value.String() == "0"
	case float64:
		return value == 0
	}
	return false
}

// hongguoxinCleanURL 清掉上游常见的转义形式。
func hongguoxinCleanURL(value string) string {
	return strings.NewReplacer(`\/`, "/", `\u0026`, "&", "&amp;", "&").Replace(strings.TrimSpace(value))
}

// hongguoxinDecryptURL 解密被 key_seed 加密的媒体地址：
// [0]=0xa8, [2]=0x01, [3]=0x00, [4:] 为密文；key/iv = SHA512(SHA512(seed) || SALT) 前 32 字节。
func hongguoxinDecryptURL(value string, seed []byte) (string, error) {
	raw, err := decodeHongguoBase64(value)
	if err != nil || len(raw) < 5 {
		return "", errors.New("红果新片源密文无效")
	}
	if raw[0] != 0xa8 || raw[2] != 0x01 || raw[3] != 0x00 {
		return "", errors.New("红果新片源密文头无效")
	}
	length := ((len(raw) - 4) / aes.BlockSize) * aes.BlockSize
	if length == 0 {
		return "", errors.New("红果新片源密文不完整")
	}
	derived := hongguoxinDeriveKeyIV(seed)
	block, err := aes.NewCipher(derived[:16])
	if err != nil {
		return "", err
	}
	plain := make([]byte, length)
	cipher.NewCBCDecrypter(block, derived[16:32]).CryptBlocks(plain, raw[4:4+length])
	end := len(plain)
	if end > 0 {
		if padding := int(plain[end-1]); padding >= 1 && padding <= aes.BlockSize && padding <= end {
			end -= padding
		}
	}
	for end > 0 && plain[end-1] == 0 {
		end--
	}
	return string(plain[:end]), nil
}

// hongguoxinCheckedURL 白名单校验：必须 https、无用户信息、端口 443，且 host 属于给定域名。
func hongguoxinCheckedURL(value string, domains []string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "", errors.New("红果新地址格式无效")
	}
	if parsed.Scheme != "https" || parsed.User != nil {
		return "", errors.New("红果新地址不受支持")
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return "", errors.New("红果新地址端口不受支持")
	}
	host := strings.ToLower(parsed.Hostname())
	for _, domain := range domains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return parsed.String(), nil
		}
	}
	return "", errors.New("红果新地址域名不在白名单")
}
