package core

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestHongguoxinPlaybackURLCanonical(t *testing.T) {
	address := hongguoxinPlaybackURL(hongguoxinDevice{DeviceID: "111", IID: "222", CDID: "333"}, 1700000000000)
	if !strings.HasPrefix(address, hongguoxinVideoAPI+"?") {
		t.Fatalf("unexpected prefix: %s", address)
	}
	query := strings.TrimPrefix(address, hongguoxinVideoAPI+"?")
	// * 必须编成 %2A，上游按 python quote 语义解析。
	if !strings.Contains(query, "resolution=1280%2A2772") {
		t.Fatalf("resolution not encoded as expected: %s", query)
	}
	for _, field := range []string{"device_id=111", "iid=222", "cdid=333", "_rticket=1700000000000"} {
		if !strings.Contains(query, field) {
			t.Fatalf("missing %s in %s", field, query)
		}
	}
	// 注册身份必须排在基础参数之后（上游对字段序敏感）。
	if strings.Index(query, "device_id=") < strings.Index(query, "ac=wifi") {
		t.Fatalf("device identity placed before base params: %s", query)
	}
}

func TestHongguoxinPlaybackPayload(t *testing.T) {
	body, err := json.Marshal(hongguoxinPlaybackPayload("7693917197759958040"))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	mixed, _ := payload["mixed_video_id_map"].(map[string]any)
	ids, _ := mixed["1004"].([]any)
	if len(ids) != 1 || ids[0] != "7693917197759958040" {
		t.Fatalf("mixed_video_id_map mismatch: %v", mixed)
	}
	biz, _ := payload["biz_param"].(map[string]any)
	if biz["video_platform"].(float64) != 1024 || biz["device_level"].(float64) != 1 {
		t.Fatalf("biz_param mismatch: %v", biz)
	}
	if _, found := payload["dr_scene"]; found {
		t.Fatal("payload must not carry dr_scene")
	}
}

func TestHongguoxinEncryptPayloadLayout(t *testing.T) {
	random := bytes.Repeat([]byte{7}, 32)
	body, err := hongguoxinEncryptPayload([]byte(`{"magic_tag":"ss_app_log"}`), random)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(body, hongguoxinMagic) {
		t.Fatalf("magic prefix mismatch: %x", body[:6])
	}
	if !bytes.Equal(body[6:38], random) {
		t.Fatal("random block mismatch")
	}
	ciphertext := body[38:]
	if len(ciphertext)%aes.BlockSize != 0 || len(ciphertext) == 0 {
		t.Fatalf("ciphertext not block aligned: %d", len(ciphertext))
	}
	// 用同一套派生参数解密，应还原成 SHA512(gzip) || gzip。
	derived := hongguoxinDeriveKeyIV(random)
	block, err := aes.NewCipher(derived[:16])
	if err != nil {
		t.Fatal(err)
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, derived[16:32]).CryptBlocks(plain, ciphertext)
	padded, err := pkcs7Unpad(plain, aes.BlockSize)
	if err != nil {
		t.Fatal(err)
	}
	if len(padded) < 64 {
		t.Fatalf("round trip payload too short: %d", len(padded))
	}
}

func TestHongguoxinSpadeURLRoundTrip(t *testing.T) {
	seed := []byte("seed-material-for-test")
	target := "https://v3.qznovelvod.com/abc/def.mp4?x=1"
	derived := hongguoxinDeriveKeyIV(seed)
	block, err := aes.NewCipher(derived[:16])
	if err != nil {
		t.Fatal(err)
	}
	padded := pkcs7Pad([]byte(target), aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, derived[16:32]).CryptBlocks(ciphertext, padded)
	raw := append([]byte{0xa8, 0x00, 0x01, 0x00}, ciphertext...)
	encoded := base64.StdEncoding.EncodeToString(raw)

	decoded, err := hongguoxinDecryptURL(encoded, seed)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != target {
		t.Fatalf("decrypted %q, want %q", decoded, target)
	}
	if _, err := hongguoxinCheckedURL(decoded, hongguoxinVODDomains); err != nil {
		t.Fatalf("whitelist rejected upstream media host: %v", err)
	}
}

func TestHongguoxinURLWhitelist(t *testing.T) {
	for _, address := range []string{
		"http://v.qznovelvod.com/a.mp4",
		"https://evil.example.com/a.mp4",
		"https://qznovelvod.com.evil.test/a.mp4",
		"https://v.qznovelvod.com:8443/a.mp4",
	} {
		if _, err := hongguoxinCheckedURL(address, hongguoxinVODDomains); err == nil {
			t.Fatalf("whitelist accepted %s", address)
		}
	}
}

func TestHongguoxinCodecAndQuality(t *testing.T) {
	cases := map[string]int{"h264": 0, "avc1": 0, "hevc": 1, "bytevc1": 1, "bytevc2": 2, "": 2}
	for codec, want := range cases {
		if got := hongguoxinCodecRank(codec); got != want {
			t.Fatalf("rank(%q)=%d, want %d", codec, got, want)
		}
	}
	if got := hongguoxinPixels("1080p"); got != 1080 {
		t.Fatalf("pixels(1080p)=%d", got)
	}
	if got := hongguoxinPixels(""); got != 0 {
		t.Fatalf("pixels(empty)=%d", got)
	}
}

func TestHongguoxinDramaRemap(t *testing.T) {
	raw := Drama{ID: "hongguo:800001", Source: sourceHongguo, SourceID: "800001", Title: "合成的剧", ChannelName: "红果"}
	converted := hongguoxinDrama(raw)
	if converted.ID != "hongguoxin:800001" || converted.Source != sourceHongguoNew {
		t.Fatalf("unexpected remap: %+v", converted)
	}
	if converted.Title != raw.Title {
		t.Fatal("title should be preserved")
	}
	source, sourceID, ok := splitProviderDramaID(converted.ID)
	if !ok || source != sourceHongguoNew || sourceID != "800001" {
		t.Fatalf("remapped id not splittable: %s", converted.ID)
	}
	chapters := hongguoxinChapters("800001", []Chapter{
		{ID: "hongguo:800001:900001", Source: sourceHongguo, Title: "第1集", VideoURL: "hongguo-cenc://900001"},
	})
	if len(chapters) != 1 || chapters[0].VideoURL != "hongguoxin-cenc://900001" ||
		chapters[0].ID != "hongguoxin:800001:900001" || chapters[0].Source != sourceHongguoNew {
		t.Fatalf("unexpected chapters: %+v", chapters)
	}
}
