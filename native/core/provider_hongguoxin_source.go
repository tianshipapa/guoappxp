package core

// 红果新（sourceHongguoNew）的目录 / 详情 / 搜索 / 封面。
//
// 这些接口与「红果」完全一致（仍走上游签名链路，2026-10-27 的上游变化只影响取播放地址），
// 因此这里直接复用红果的实现，只把返回结果里的站源标记与 ID 前缀改成红果新，
// 让两个源在剧库、追剧、下载记录里彼此独立、可并存。

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// hongguoxinCatalogLimit 单次分类请求条数（与上游 landpage 接口一致）。
const hongguoxinCatalogLimit = 54

// hongguoxinDrama 把红果条目改挂到红果新：ID / Source 换成新源，其余元数据保持不变。
func hongguoxinDrama(drama Drama) Drama {
	_, sourceID, ok := splitProviderDramaID(drama.ID)
	if !ok || sourceID == "" {
		sourceID = strings.TrimSpace(drama.SourceID)
	}
	if sourceID == "" {
		return Drama{}
	}
	drama.SourceID = sourceID
	drama.ID = providerDramaID(sourceHongguoNew, sourceID)
	drama.Source = sourceHongguoNew
	drama.ChannelName = "红果新"
	return drama
}

// hongguoxinChapters 把红果章节改挂到红果新，并把取流前缀换成 hongguoxin-cenc://。
func hongguoxinChapters(seriesID string, chapters []Chapter) []Chapter {
	out := make([]Chapter, 0, len(chapters))
	for _, chapter := range chapters {
		videoID := strings.TrimPrefix(chapter.VideoURL, "hongguo-cenc://")
		if !hongguoNumericID.MatchString(videoID) {
			continue
		}
		chapter.ID = providerChapterID(sourceHongguoNew, seriesID, videoID)
		chapter.Source = sourceHongguoNew
		chapter.VideoURL = "hongguoxin-cenc://" + videoID
		out = append(out, chapter)
	}
	return out
}

// fetchHongguoxinCatalogPage 按页取分类列表。
// 红果的游标式翻页与「红果」源共用同一份状态，会让两个源互相推进 offset，
// 所以红果新改用无状态的 offset 分页（offset = (page-1)*54），互不干扰。
func (d *Downloader) fetchHongguoxinCatalogPage(ctx context.Context, page int, category string) ([]Drama, bool, error) {
	genres := hongguoAppGenres
	if category != "" {
		genres = nil
		for _, genre := range hongguoAppGenres {
			if genre.key == category {
				genres = append(genres, genre)
			}
		}
		if len(genres) == 0 {
			return nil, false, errors.New("红果新分类无效")
		}
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * hongguoxinCatalogLimit
	if offset > 1_000_000 {
		return nil, false, nil
	}
	seen := map[string]bool{}
	var items []Drama
	hasMore := false
	var failures []error
	for _, genre := range genres {
		payload := map[string]any{
			"req_scene": genre.scene, "offset": offset, "limit": hongguoxinCatalogLimit,
			"req_type": "only_content", "need_selector_panel": false, "client_req_type": 3,
			"session_id": "", "filter_ids": "",
			"select_items": map[string]any{
				"genre": []string{genre.key}, "sort": []string{"online_time"}, "gender": []string{},
				"category_dim_theme": []string{}, "category_dim_role": []string{}, "category_dim_epoch": []string{},
				"online_time": []string{}, "creation_status": []string{},
			},
		}
		if offset > 0 {
			payload["client_req_type"] = 2
		}
		result, err := d.hongguoAppRequest(ctx, http.MethodPost, "/reading/distribution/category/landpage/v/", nil, payload)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", genre.name, err))
			continue
		}
		data := nestedMap(result, "data")
		rows := anyList(data["video_data"])
		if more, ok := data["has_more"].(bool); ok && more {
			hasMore = true
		}
		for _, row := range rows {
			drama := hongguoDramaFromAny(row, genre.name)
			if drama.ID == "" || seen[drama.ID] {
				continue
			}
			seen[drama.ID] = true
			items = append(items, hongguoxinDrama(drama))
		}
	}
	if len(items) == 0 && len(failures) > 0 {
		return nil, false, errors.Join(failures...)
	}
	if len(items) >= hongguoxinCatalogLimit {
		hasMore = true
	}
	return items, hasMore, nil
}

// fetchHongguoxinChapters 复用红果的 App 详情（同一 series_id），再改挂红果新。
func (d *Downloader) fetchHongguoxinChapters(ctx context.Context, sourceID string) (string, []Chapter, error) {
	sourceID = strings.TrimPrefix(strings.TrimSpace(sourceID), "hg-series-v1:")
	if sourceID == "" {
		return "", nil, errors.New("红果新剧集 ID 为空")
	}
	entry, err := d.hongguoAppDetail(ctx, sourceID)
	if err != nil {
		return "", nil, err
	}
	chapters := hongguoxinChapters(sourceID, entry.Chapters)
	if len(chapters) == 0 {
		return entry.Drama.DisplayTitle(), nil, errors.New("红果新详情没有返回可播放的分集")
	}
	return entry.Drama.DisplayTitle(), chapters, nil
}

// hongguoxinCover 复用红果的 video_detail 签名接口取海报。
func (d *Downloader) hongguoxinCover(ctx context.Context, seriesID string) (string, error) {
	if !hongguoNumericID.MatchString(seriesID) {
		return "", errors.New("红果新剧集 ID 无效")
	}
	result, err := d.hongguoAppRequest(ctx, http.MethodPost, "/novel/player/video_detail/v1/", nil, map[string]any{"series_id": seriesID})
	if err != nil {
		return "", err
	}
	row := nestedMap(result, "data", "video_data")
	if mapString(row, "series_id_str", "series_id") != seriesID {
		return "", errors.New("红果新详情与请求剧集不符")
	}
	return hongguoCoverAddress(mapString(row, "series_cover", "cover")), nil
}

// searchHongguoxinDramas 复用红果的搜索结果，再改挂红果新。
func (d *Downloader) searchHongguoxinDramas(ctx context.Context, keyword string) (hongguoSearchEntry, error) {
	entry, err := d.searchHongguoDramas(ctx, keyword)
	if err != nil {
		return entry, err
	}
	return hongguoxinSearchEntry(entry), nil
}

// fetchHongguoxinSearchPage 复用红果的翻页搜索结果，再改挂红果新。
func (d *Downloader) fetchHongguoxinSearchPage(ctx context.Context, keyword string, page int) (hongguoSearchEntry, error) {
	entry, err := d.fetchHongguoSearchPage(ctx, keyword, page)
	if err != nil {
		return entry, err
	}
	return hongguoxinSearchEntry(entry), nil
}

func hongguoxinSearchEntry(entry hongguoSearchEntry) hongguoSearchEntry {
	dramas := make([]Drama, 0, len(entry.Dramas))
	for _, drama := range entry.Dramas {
		if converted := hongguoxinDrama(drama); converted.ID != "" {
			dramas = append(dramas, converted)
		}
	}
	entry.Dramas = dramas
	return entry
}
