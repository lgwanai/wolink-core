package plugins

import (
	"bufio"
	"io"
	"strings"
)

// StreamWrapper 流式响应包装器
type StreamWrapper struct {
	reader   *bufio.Reader
	closer   io.Closer
	closed   bool
	buffer   []byte
	hasData  bool
}

// NewStreamWrapper 创建流式包装器
func NewStreamWrapper(rc io.ReadCloser) *StreamWrapper {
	return &StreamWrapper{
		reader: bufio.NewReader(rc),
		closer: rc,
		closed: false,
	}
}

// Read 实现io.Reader接口，提供真正的流式读取
func (sw *StreamWrapper) Read(p []byte) (n int, err error) {
	if sw.closed {
		return 0, io.EOF
	}

	// 如果缓冲区有数据，先返回缓冲区的数据
	if sw.hasData {
		n = copy(p, sw.buffer)
		if n < len(sw.buffer) {
			// 如果p太小，保留剩余数据
			sw.buffer = sw.buffer[n:]
		} else {
			// 缓冲区数据已全部复制
			sw.buffer = nil
			sw.hasData = false
		}
		return n, nil
	}

	// 逐行读取数据
	line, isPrefix, err := sw.reader.ReadLine()
	if err != nil {
		return 0, err
	}

	// 处理截断的行
	for isPrefix {
		var nextPart []byte
		nextPart, isPrefix, err = sw.reader.ReadLine()
		if err != nil {
			return 0, err
		}
		line = append(line, nextPart...)
	}

	// 跳过空行
	if len(line) == 0 {
		return sw.Read(p) // 递归读取下一行
	}

	// 跳过非data行
	lineStr := string(line)
	if !strings.HasPrefix(lineStr, "data:") {
		return sw.Read(p) // 递归读取下一行
	}

	// 添加换行符
	lineWithNewline := append(line, '\n')

	// 复制到输出缓冲区
	n = copy(p, lineWithNewline)
	if n < len(lineWithNewline) {
		// 如果p太小，保存剩余数据到缓冲区
		sw.buffer = lineWithNewline[n:]
		sw.hasData = true
	}

	return n, nil
}

// Close 关闭流
func (sw *StreamWrapper) Close() error {
	if sw.closed {
		return nil
	}
	sw.closed = true
	return sw.closer.Close()
}