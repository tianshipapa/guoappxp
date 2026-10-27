package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNativeSearchMergeSameDramaAcrossSources(t *testing.T) {
	merged := nativeSearchMerge([]nativeDrama{
		{ID: "hongguo:1", Source: "hongguo", Title: "夜色温柔", Episodes: 24, OnlineDate: "2025-03-01"},
		{ID: "yeguo:9", Source: "yeguo", Title: "夜色温柔", Episodes: 24, OnlineDate: "2025-03-01", Cover: "https://example.com/a.jpg"},
		{ID: "dsd:5", Source: "dsd", Title: "夜色温柔", Episodes: 24, OnlineDate: "2025-03-01"},
	}, true)
	if len(merged) != 1 {
		t.Fatalf("期望归一为 1 条，实际 %d 条", len(merged))
	}
	// 带封面的条目信息更完整，应被选为主条目。
	if merged[0].Source != "yeguo" {
		t.Fatalf("期望主条目来自 yeguo，实际 %s", merged[0].Source)
	}
	if len(merged[0].Peers) != 2 {
		t.Fatalf("期望 2 个其它站源副本，实际 %d", len(merged[0].Peers))
	}
}

func TestNativeSearchMergeKeepsDifferentTitles(t *testing.T) {
	merged := nativeSearchMerge([]nativeDrama{
		{ID: "hongguo:1", Source: "hongguo", Title: "夜色温柔"},
		{ID: "yeguo:2", Source: "yeguo", Title: "夜色微凉"},
	}, true)
	if len(merged) != 2 {
		t.Fatalf("标题不同不应合并，实际 %d 条", len(merged))
	}
}

func TestNativeSearchMergeKeepsDifferentYears(t *testing.T) {
	merged := nativeSearchMerge([]nativeDrama{
		{ID: "hongguo:1", Source: "hongguo", Title: "夜色温柔", OnlineDate: "2023-05-01"},
		{ID: "yeguo:2", Source: "yeguo", Title: "夜色温柔", OnlineDate: "2025-05-01"},
	}, true)
	if len(merged) != 2 {
		t.Fatalf("年份不同不应合并，实际 %d 条", len(merged))
	}
}

func TestNativeSearchMergeKeepsFarApartEpisodeCounts(t *testing.T) {
	merged := nativeSearchMerge([]nativeDrama{
		{ID: "hongguo:1", Source: "hongguo", Title: "夜色温柔", Episodes: 12},
		{ID: "yeguo:2", Source: "yeguo", Title: "夜色温柔", Episodes: 80},
	}, true)
	if len(merged) != 2 {
		t.Fatalf("集数差异过大不应合并，实际 %d 条", len(merged))
	}
}

func TestNativeSearchMergeToleratesMissingEpisodeCount(t *testing.T) {
	merged := nativeSearchMerge([]nativeDrama{
		{ID: "hongguo:1", Source: "hongguo", Title: "夜色温柔", Episodes: 0},
		{ID: "yeguo:2", Source: "yeguo", Title: "夜色温柔", Episodes: 80},
	}, true)
	if len(merged) != 1 {
		t.Fatalf("集数缺失时应合并，实际 %d 条", len(merged))
	}
}

func TestNativeSearchMergeNormalizesPunctuation(t *testing.T) {
	merged := nativeSearchMerge([]nativeDrama{
		{ID: "hongguo:1", Source: "hongguo", Title: "夜色温柔：第一季"},
		{ID: "yeguo:2", Source: "yeguo", Title: "夜色温柔 第一季"},
	}, true)
	if len(merged) != 1 {
		t.Fatalf("标点/空格差异应归一合并，实际 %d 条", len(merged))
	}
}

// 流式搜索的中间快照不能重排已有条目，否则用户正看着的一行会突然跳走。
func TestNativeSearchMergeKeepsArrivalOrderWhileStreaming(t *testing.T) {
	merged := nativeSearchMerge([]nativeDrama{
		{ID: "hongguo:1", Source: "hongguo", Title: "甲"},
		{ID: "yeguo:2", Source: "yeguo", Title: "乙", Cover: "https://example.com/b.jpg", Episodes: 30},
	}, false)
	if len(merged) != 2 || merged[0].Title != "甲" || merged[1].Title != "乙" {
		t.Fatalf("流式快照应保持到达顺序，实际 %v", merged)
	}
}

func TestNativeSearchSourceRejectsUnsupportedSource(t *testing.T) {
	if !nativeSourceSupportsSearch(sourceHongguo) {
		t.Fatal("红果应支持在线搜索")
	}
	if nativeSourceSupportsSearch("definitely-not-a-source") {
		t.Fatal("未知站源不应判定为支持在线搜索")
	}
}

// 一个站源先返回、另一个还卡着时，快照要立刻给出已到的结果，
// 并且 pending 里留着没回来的站源，finished 为 false。
func TestNativeSearchSessionStreamsPartialResults(t *testing.T) {
	session := &nativeSearchSession{
		id:       "test",
		query:    "夜色温柔",
		order:    []string{"hongguo", "yeguo"},
		items:    map[string][]nativeDrama{},
		done:     map[string]bool{},
		failures: map[string]string{},
		started:  time.Now(),
		cancel:   func() {},
	}
	session.finish("hongguo", []nativeDrama{{ID: "hongguo:1", Source: "hongguo", Title: "夜色温柔", Episodes: 24}}, nil)
	partial := session.snapshot()
	if partial.Finished {
		t.Fatal("还有站源在搜，不应标记为已结束")
	}
	if len(partial.Items) != 1 {
		t.Fatalf("应立刻返回已到达的 1 条结果，实际 %d 条", len(partial.Items))
	}
	if len(partial.Pending) != 1 || partial.Pending[0] != "yeguo" {
		t.Fatalf("待返回站源应为 yeguo，实际 %v", partial.Pending)
	}
	if partial.Done != 1 || partial.Total != 2 {
		t.Fatalf("进度应为 1/2，实际 %d/%d", partial.Done, partial.Total)
	}

	session.finish("yeguo", nil, errors.New("站源开小差"))
	final := session.snapshot()
	if !final.Finished {
		t.Fatal("所有站源都有结论后应标记为已结束")
	}
	if final.Warning == "" {
		t.Fatal("有站源失败时应给出提示")
	}
	if len(final.Failures) != 1 || final.Failures["yeguo"] == "" {
		t.Fatalf("失败站源应记录原因，实际 %v", final.Failures)
	}
}

// 同一站源重复落地只认第一次，避免 goroutine panic 兜底把成功结果改写掉。
func TestNativeSearchSessionFinishIsIdempotent(t *testing.T) {
	session := &nativeSearchSession{
		id:       "test",
		order:    []string{"hongguo"},
		items:    map[string][]nativeDrama{},
		done:     map[string]bool{},
		failures: map[string]string{},
		started:  time.Now(),
		cancel:   func() {},
	}
	session.finish("hongguo", []nativeDrama{{ID: "hongguo:1", Source: "hongguo", Title: "夜色温柔"}}, nil)
	session.finish("hongguo", nil, context.Canceled)
	snapshot := session.snapshot()
	if len(snapshot.Items) != 1 {
		t.Fatalf("成功结果不应被后续失败覆盖，实际 %d 条", len(snapshot.Items))
	}
	if len(snapshot.Failures) != 0 {
		t.Fatalf("重复落地不应记录失败，实际 %v", snapshot.Failures)
	}
}
