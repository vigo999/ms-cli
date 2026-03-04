package context

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/vigo999/ms-cli/integrations/llm"
)

// Tokenizer 提供 Token 估算功能
// 注意：这是基于启发式的估算，不是精确的 tiktoken 计算
type Tokenizer struct {
	// 配置参数
	charsPerToken    float64 // 平均每个 Token 的字符数
	wordsPerToken    float64 // 平均每个 Token 的单词数
	codeCharsPerToken float64 // 代码的平均每个 Token 字符数
}

// NewTokenizer 创建新的 Tokenizer
func NewTokenizer() *Tokenizer {
	return &Tokenizer{
		charsPerToken:     4.0,  // 英文文本约 4 字符/Token
		wordsPerToken:     0.75, // 约 0.75 单词/Token
		codeCharsPerToken: 3.5,  // 代码通常更密集
	}
}

// SetCharsPerToken 设置字符/Token 比例
func (t *Tokenizer) SetCharsPerToken(ratio float64) {
	t.charsPerToken = ratio
}

// EstimateText 估算纯文本的 Token 数
func (t *Tokenizer) EstimateText(text string) int {
	if text == "" {
		return 0
	}

	// 基于字符数的估算
	charCount := utf8.RuneCountInString(text)
	return int(float64(charCount) / t.charsPerToken)
}

// EstimateCode 估算代码的 Token 数
func (t *Tokenizer) EstimateCode(code string) int {
	if code == "" {
		return 0
	}

	// 代码通常更密集，使用更小的比例
	charCount := utf8.RuneCountInString(code)
	return int(float64(charCount) / t.codeCharsPerToken)
}

// EstimateMessage 估算单个消息的 Token 数
func (t *Tokenizer) EstimateMessage(msg llm.Message) int {
	if msg.Content == "" && len(msg.ToolCalls) == 0 {
		return 0
	}

	total := 0

	// 消息基础开销（角色、格式等）
	total += 4

	// 内容 Token
	total += t.EstimateText(msg.Content)

	// ToolCalls Token
	for _, tc := range msg.ToolCalls {
		total += t.estimateToolCall(tc)
	}

	return total
}

// EstimateMessages 估算多个消息的 Token 数
func (t *Tokenizer) EstimateMessages(msgs []llm.Message) int {
	total := 0
	for _, msg := range msgs {
		total += t.EstimateMessage(msg)
	}
	// 每个对话有额外的格式开销
	if len(msgs) > 0 {
		total += 2
	}
	return total
}

// estimateToolCall 估算 ToolCall 的 Token 数
func (t *Tokenizer) estimateToolCall(tc llm.ToolCall) int {
	total := 0
	// Tool call ID
	total += len(tc.ID) / 4
	// Function name
	total += len(tc.Function.Name) / 4
	// Arguments
	total += t.EstimateText(string(tc.Function.Arguments))
	// 格式开销
	total += 4
	return total
}

// EstimateWithDetails 返回详细的估算信息
type EstimateDetails struct {
	TotalTokens    int
	ContentTokens  int
	ToolCallTokens int
	OverheadTokens int
}

// EstimateMessageWithDetails 估算消息并返回详细信息
func (t *Tokenizer) EstimateMessageWithDetails(msg llm.Message) EstimateDetails {
	contentTokens := t.EstimateText(msg.Content)
	toolCallTokens := 0
	for _, tc := range msg.ToolCalls {
		toolCallTokens += t.estimateToolCall(tc)
	}
	overhead := 4
	total := contentTokens + toolCallTokens + overhead

	return EstimateDetails{
		TotalTokens:    total,
		ContentTokens:  contentTokens,
		ToolCallTokens: toolCallTokens,
		OverheadTokens: overhead,
	}
}

// SimpleTokenizer 简单的 Tokenizer 实现
type SimpleTokenizer struct {
	// 用于中文的估算
	chineseCharsPerToken float64
	englishCharsPerToken float64
}

// NewSimpleTokenizer 创建简单的 Tokenizer
func NewSimpleTokenizer() *SimpleTokenizer {
	return &SimpleTokenizer{
		chineseCharsPerToken: 2.0, // 中文约 2 字符/Token
		englishCharsPerToken: 4.0, // 英文约 4 字符/Token
	}
}

// Estimate 估算文本的 Token 数
func (t *SimpleTokenizer) Estimate(text string) int {
	if text == "" {
		return 0
	}

	// 分离中英文
	chineseCount := countChineseChars(text)
	englishCount := len(text) - chineseCount

	tokens := 0
	tokens += int(float64(chineseCount) / t.chineseCharsPerToken)
	tokens += int(float64(englishCount) / t.englishCharsPerToken)

	return tokens
}

// countChineseChars 统计中文字符数量
func countChineseChars(text string) int {
	// 匹配 CJK 字符范围
	re := regexp.MustCompile(`[\p{Han}]`)
	return len(re.FindAllString(text, -1))
}

// countWords 统计单词数量
func countWords(text string) int {
	fields := strings.Fields(text)
	return len(fields)
}

// EstimateByWords 基于单词数估算 Token
func EstimateByWords(wordCount int) int {
	// 约 0.75 tokens per word
	return int(float64(wordCount) / 0.75)
}

// GlobalTokenizer 全局 Tokenizer 实例
var GlobalTokenizer = NewTokenizer()

// Estimate 使用全局 Tokenizer 估算文本
func Estimate(text string) int {
	return GlobalTokenizer.EstimateText(text)
}

// EstimateMessage 使用全局 Tokenizer 估算消息
func EstimateMessage(msg llm.Message) int {
	return GlobalTokenizer.EstimateMessage(msg)
}

// EstimateMessages 使用全局 Tokenizer 估算多个消息
func EstimateMessages(msgs []llm.Message) int {
	return GlobalTokenizer.EstimateMessages(msgs)
}
