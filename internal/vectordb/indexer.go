package vectordb

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// Chunk 文档分块
type Chunk struct {
	ID       string
	Content  string
	Source   string // 来源文件路径
	Section  string // 所在章节标题
	Metadata map[string]string
}

// ChunkMarkdown 将 Markdown 文本按 ## 标题分块
// maxChunkSize: 每块最大字符数（默认 500）
// overlap: 重叠字符数（默认 50）
func ChunkMarkdown(content, source string, maxChunkSize, overlap int) []Chunk {
	if maxChunkSize <= 0 {
		maxChunkSize = 500
	}
	if overlap <= 0 {
		overlap = 50
	}

	// 提取 frontmatter 中的 tags
	tags := extractFrontmatterTags(content)

	// 按 ## 标题分段
	sections := splitByHeadings(content)

	var chunks []Chunk
	for _, sec := range sections {
		text := strings.TrimSpace(sec.content)
		if len(text) == 0 {
			continue
		}

		// 如果段落小于 maxChunkSize，整段作为一个 chunk
		if len(text) <= maxChunkSize {
			chunks = append(chunks, Chunk{
				ID:      hashID(source, sec.heading, 0),
				Content: text,
				Source:  source,
				Section: sec.heading,
				Metadata: map[string]string{
					"source":  source,
					"section": sec.heading,
					"tags":    strings.Join(tags, ","),
				},
			})
			continue
		}

		// 超长段落按 maxChunkSize 滑窗分块
		for i := 0; i < len(text); i += maxChunkSize - overlap {
			end := i + maxChunkSize
			if end > len(text) {
				end = len(text)
			}
			chunk := text[i:end]
			chunks = append(chunks, Chunk{
				ID:      hashID(source, sec.heading, i),
				Content: chunk,
				Source:  source,
				Section: sec.heading,
				Metadata: map[string]string{
					"source":  source,
					"section": sec.heading,
					"tags":    strings.Join(tags, ","),
				},
			})
			if end >= len(text) {
				break
			}
		}
	}

	return chunks
}

type section struct {
	heading string
	content string
}

func splitByHeadings(content string) []section {
	lines := strings.Split(content, "\n")
	var sections []section
	current := section{heading: "intro"}
	var buf strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			// 保存前一段
			current.content = buf.String()
			if current.content != "" {
				sections = append(sections, current)
			}
			current = section{heading: strings.TrimPrefix(line, "## ")}
			buf.Reset()
		} else {
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}
	// 最后一段
	current.content = buf.String()
	if current.content != "" {
		sections = append(sections, current)
	}

	return sections
}

func extractFrontmatterTags(content string) []string {
	if !strings.HasPrefix(content, "---") {
		return nil
	}
	end := strings.Index(content[3:], "---")
	if end < 0 {
		return nil
	}
	fm := content[3 : end+3]
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "tags:") {
			tagStr := strings.TrimPrefix(line, "tags:")
			tagStr = strings.Trim(tagStr, " []")
			var tags []string
			for _, t := range strings.Split(tagStr, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, t)
				}
			}
			return tags
		}
	}
	return nil
}

func hashID(source, section string, offset int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", source, section, offset)))
	return fmt.Sprintf("%x", h[:8])
}
