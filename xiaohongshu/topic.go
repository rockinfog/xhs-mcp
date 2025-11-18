package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/errors"
)

// TopicAction 表示话题页面动作
type TopicAction struct {
	page *rod.Page
}

// TopicFeed 表示话题页面的Feed数据
type TopicFeed struct {
	Type            string               `json:"type"`
	Title           string               `json:"title"`
	Desc            string               `json:"desc"`
	User            TopicFeedUser        `json:"user"`
	InteractionInfo TopicInteractionInfo `json:"interactionInfo"`
	CreateTime      int64                `json:"createTime"`
	CursorScore     string               `json:"cursorScore"`
}

// TopicFeedUser 表示话题Feed中的用户信息
type TopicFeedUser struct {
	Nickname    string `json:"nickname"`
	AvatarURL   string `json:"avatarUrl"`
	IsForbidden bool   `json:"isForbidden"`
}

// TopicInteractionInfo 表示话题Feed的互动信息
type TopicInteractionInfo struct {
	LikeText    string `json:"likeText"`
	CollectText string `json:"collectText"`
	CommentText string `json:"commentText"`
}

// TopicInfo 表示话题信息
type TopicInfo struct {
	Name              string `json:"name"`
	Desc              string `json:"desc"`
	ViewNumText       string `json:"viewNumText"`           // 浏览量文本
	DiscussCommentNum string `json:"discussCommentNumText"` // 讨论数文本
}

// TopicResponse 表示话题页面的完整响应
type TopicResponse struct {
	Topic TopicInfo   `json:"topic"`
	Feeds []TopicFeed `json:"feeds"`
	Count int         `json:"count"`
}

// NewTopicAction 创建话题页面动作
func NewTopicAction(page *rod.Page) *TopicAction {
	return &TopicAction{page: page}
}

// GetTopicFeeds 获取话题页面的Feed列表数据
func (t *TopicAction) GetTopicFeeds(ctx context.Context, topicID string) (*TopicResponse, error) {
	page := t.page.Context(ctx).Timeout(60 * time.Second)

	// 构建话题页 URL
	url := makeTopicURL(topicID)

	logrus.Infof("打开话题页面: %s", url)

	// 导航到话题页
	page.MustNavigate(url)
	page.MustWaitStable()

	// 等待基本的页面数据结构加载
	page.MustWait(`() => window.__INITIAL_STATE__ !== undefined`)

	// 等待一段时间让数据完全加载
	time.Sleep(2 * time.Second)

	// 提取话题信息 (从 topicData.pageInfo 中获取)
	topicResult := page.MustEval(`() => {
		if (window.__INITIAL_STATE__ &&
		    window.__INITIAL_STATE__.topic &&
		    window.__INITIAL_STATE__.topic.topicData) {
			const topicData = window.__INITIAL_STATE__.topic.topicData;
			const data = topicData.value !== undefined ? topicData.value : topicData._value;
			if (data && data.pageInfo) {
				return JSON.stringify(data.pageInfo);
			}
		}
		return "";
	}`).String()

	if topicResult == "" {
		// 输出完整的页面结构用于调试
		fullDebug := page.MustEval(`() => {
			if (!window.__INITIAL_STATE__) return "window.__INITIAL_STATE__ 不存在";
			if (!window.__INITIAL_STATE__.topic) return "window.__INITIAL_STATE__.topic 不存在, 顶层keys: " + Object.keys(window.__INITIAL_STATE__).join(", ");
			if (!window.__INITIAL_STATE__.topic.topicData) return "window.__INITIAL_STATE__.topic.topicData 不存在, topic keys: " + Object.keys(window.__INITIAL_STATE__.topic).join(", ");
			return "topicData 数据为空";
		}`).String()
		logrus.Errorf("话题信息获取失败: %s", fullDebug)
		return nil, fmt.Errorf("未找到话题信息: %s", fullDebug)
	}

	var topicInfo TopicInfo
	if err := json.Unmarshal([]byte(topicResult), &topicInfo); err != nil {
		logrus.Errorf("解析话题信息失败，原始数据: %s", topicResult)
		return nil, fmt.Errorf("解析话题信息失败: %w", err)
	}

	logrus.Debugf("解析后的话题信息: %+v", topicInfo)

	// 提取Feed列表数据 (实际字段名是 topicNotes)
	feedsResult := page.MustEval(`() => {
		if (window.__INITIAL_STATE__ &&
		    window.__INITIAL_STATE__.topic &&
		    window.__INITIAL_STATE__.topic.topicNotes) {
			const topicNotes = window.__INITIAL_STATE__.topic.topicNotes;
			const feedsData = topicNotes.value !== undefined ? topicNotes.value : topicNotes._value;
			if (feedsData) {
				return JSON.stringify(feedsData);
			}
		}
		return "";
	}`).String()

	if feedsResult == "" {
		// 输出调试信息
		debugInfo := page.MustEval(`() => {
			if (!window.__INITIAL_STATE__) return "window.__INITIAL_STATE__ 不存在";
			if (!window.__INITIAL_STATE__.topic) return "window.__INITIAL_STATE__.topic 不存在";
			if (!window.__INITIAL_STATE__.topic.topicNotes) return "window.__INITIAL_STATE__.topic.topicNotes 不存在";
			return "topicNotes 数据为空";
		}`).String()
		logrus.Errorf("话题Feed列表获取失败: %s", debugInfo)
		return nil, errors.ErrNoFeeds
	}

	var feeds []TopicFeed
	if err := json.Unmarshal([]byte(feedsResult), &feeds); err != nil {
		// 输出前 500 字符用于调试
		if len(feedsResult) > 500 {
			logrus.Errorf("解析Feed数据失败，原始数据(前500字符): %s...", feedsResult[:500])
		} else {
			logrus.Errorf("解析Feed数据失败，原始数据: %s", feedsResult)
		}
		return nil, fmt.Errorf("解析Feed数据失败: %w", err)
	}

	logrus.Debugf("成功解析 %d 个话题Feed", len(feeds))

	response := &TopicResponse{
		Topic: topicInfo,
		Feeds: feeds,
		Count: len(feeds),
	}

	return response, nil
}

func makeTopicURL(topicID string) string {
	return fmt.Sprintf("https://www.xiaohongshu.com/topic/normal/%s", topicID)
}
