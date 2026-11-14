package main

import (
	"strings"
	"testing"
)

// 红果书籍 ID 的前缀处理必须幂等。
//
// 书库快照与分页切片两条路径都会调用 abHongguoPrefix，同一条记录会被加多次。
// 之前不做判断，ID 变成 hg:hg:hg:...，目录接口拿到无效 ID，前端永远停在
// 「正在加载目录…」且无法播放。
func TestAbHongguoPrefixIsIdempotent(t *testing.T) {
	raw := []any{
		map[string]any{"id": "7585573146086870040", "title": "军神出狱"},
	}
	once := abHongguoPrefix(raw)
	if got := once[0]["id"]; got != "hg:7585573146086870040" {
		t.Fatalf("首次加前缀 = %q", got)
	}
	// 再跑两遍，结果必须与第一次完全相同。
	twice := abHongguoPrefix([]any{once[0]})
	thrice := abHongguoPrefix([]any{twice[0]})
	if got := thrice[0]["id"]; got != "hg:7585573146086870040" {
		t.Fatalf("重复加前缀后 ID 漂移: %q（前缀必须幂等）", got)
	}
	if strings.Contains(strings.TrimPrefix(thrice[0]["id"].(string), "hg:"), "hg:") {
		t.Fatalf("ID 里残留多余前缀: %q", thrice[0]["id"])
	}
}

// 无论输入带几个前缀，规范化后都只保留一个。
func TestAbHongguoBookIDNormalizes(t *testing.T) {
	cases := map[string]string{
		"123":                 "hg:123",
		"hg:123":              "hg:123",
		"hg:hg:hg:123":        "hg:123",
		"  hg:456  ":          "hg:456",
		"7585573146086870040": "hg:7585573146086870040",
	}
	for input, want := range cases {
		if got := abHongguoBookID(input); got != want {
			t.Fatalf("abHongguoBookID(%q) = %q, want %q", input, got, want)
		}
	}
}
