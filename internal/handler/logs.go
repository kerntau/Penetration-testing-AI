package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"penetration-testing-ai/internal/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// LogHandler 提供运行日志的只读查看能力（系统设置 → 用量与运维）。
type LogHandler struct {
	dir    string
	logger *zap.Logger
}

// NewLogHandler 创建运行日志处理器。目录取自 log.diagnostic_dir，缺省为 log。
func NewLogHandler(cfg *config.Config, logger *zap.Logger) *LogHandler {
	dir := ""
	if cfg != nil {
		dir = strings.TrimSpace(cfg.Log.DiagnosticDir)
	}
	if dir == "" {
		dir = "log"
	}
	return &LogHandler{dir: dir, logger: logger}
}

const (
	defaultLogLines = 200
	maxLogLines     = 2000
	maxLogFileBytes = 4 << 20 // 4 MiB，避免一次性读入超大文件
)

// Recent 返回可选的日志文件列表与指定文件的最后若干行。
func (h *LogHandler) Recent(c *gin.Context) {
	lines := defaultLogLines
	if raw := strings.TrimSpace(c.Query("lines")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			lines = n
		}
	}
	if lines > maxLogLines {
		lines = maxLogLines
	}

	files, err := h.listFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取日志目录失败: " + err.Error()})
		return
	}

	name := strings.TrimSpace(c.Query("file"))
	if name == "" && len(files) > 0 {
		name = files[0]
	}
	if name != "" && !isPlainLogName(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的日志文件名"})
		return
	}

	content := make([]string, 0, lines)
	truncated := false
	if name != "" {
		content, truncated, err = h.tailFile(filepath.Join(h.dir, name), lines)
		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "日志文件不存在"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "读取日志失败: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"dir":       h.dir,
		"files":     files,
		"file":      name,
		"lines":     content,
		"truncated": truncated,
	})
}

// listFiles 按修改时间倒序返回目录下的 .log 文件。
func (h *LogHandler) listFiles() ([]string, error) {
	entries, err := os.ReadDir(h.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	type item struct {
		name string
		mod  int64
	}
	var items []item
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		items = append(items, item{name: entry.Name(), mod: info.ModTime().UnixNano()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].mod > items[j].mod })
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.name)
	}
	return names, nil
}

// tailFile 读取文件末尾的若干行，超过 maxLogFileBytes 时只读尾部。
func (h *LogHandler) tailFile(path string, lines int) ([]string, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return nil, false, err
	}
	size := info.Size()
	offset := int64(0)
	if size > maxLogFileBytes {
		offset = size - maxLogFileBytes
	}
	if _, err := file.Seek(offset, 0); err != nil {
		return nil, false, err
	}
	buf := make([]byte, size-offset)
	if _, err := file.Read(buf); err != nil && len(buf) == 0 {
		return nil, false, err
	}

	text := string(buf)
	if offset > 0 {
		// 丢弃被截断的首行，保证返回的每行都是完整的。
		if idx := strings.IndexByte(text, '\n'); idx >= 0 {
			text = text[idx+1:]
		}
	}
	all := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(all) == 1 && all[0] == "" {
		return []string{}, offset > 0, nil
	}
	truncated := offset > 0
	if len(all) > lines {
		all = all[len(all)-lines:]
		truncated = true
	}
	return all, truncated, nil
}

// isPlainLogName 只接受目录内的普通文件名，防止路径穿越。
func isPlainLogName(name string) bool {
	if name != filepath.Base(name) {
		return false
	}
	if strings.ContainsAny(name, `/\`) {
		return false
	}
	return strings.HasSuffix(name, ".log")
}
