package multio

import (
	"errors"
	"io"
)

// multiReadCloser 将多个 reader 组合成一个，并实现了 io.Closer 接口
type multiReadCloser struct {
	readers []io.Reader
	closers []io.Closer // 存储所有可关闭的 reader
}

// MultiReadReader 从一系列 reader 创建一个新的 multiReadCloser
func MultiReadReader(readers ...io.Reader) *multiReadCloser {
	closers := []io.Closer{}
	for _, r := range readers {
		if c, ok := r.(io.Closer); ok {
			closers = append(closers, c)
		}
	}
	return &multiReadCloser{
		readers: readers,
		closers: closers,
	}
}

// Read 实现了 io.Reader 接口
func (mc *multiReadCloser) Read(p []byte) (n int, err error) {
	for len(mc.readers) > 0 {
		n, err = mc.readers[0].Read(p)
		if n > 0 {
			return
		}
		if err == io.EOF {
			// 当前 reader 已读完，移至下一个
			mc.readers = mc.readers[1:]
		} else if err != nil {
			return n, err // 立即返回非 EOF 错误
		}
	}
	return 0, io.EOF
}

// Close 实现了 io.Closer 接口，会关闭所有内部可关闭的 reader
func (mc *multiReadCloser) Close() error {
	var errs []error
	for _, c := range mc.closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// funcCloser 包装一个 io.Reader，并为其附加一个自定义的 Close 函数。
// 它本身实现了 io.ReadCloser 接口。
type funcCloser struct {
	reader    io.Reader
	closeFunc func() error
}

// FuncCloser 创建一个新的 funcCloser 实例。
// r 是要包装的 reader，可以是一个 io.Reader 或 io.ReadCloser。
// closeFn 是当 Close() 方法被调用时要执行的自定义函数。
func FuncCloser(r io.Reader, closeFn func() error) io.ReadCloser {
	return &funcCloser{
		reader:    r,
		closeFunc: closeFn,
	}
}

// Read 直接调用底层 reader 的 Read 方法。
func (fc *funcCloser) Read(p []byte) (n int, err error) {
	return fc.reader.Read(p)
}

// Close 执行自定义的 closeFunc，如果底层 reader 也是 io.Closer，
// 则继续调用其 Close 方法，并合并所有错误。
func (fc *funcCloser) Close() error {
	var err1 error
	if fc.closeFunc != nil {
		err1 = fc.closeFunc()
	}
	var err2 error
	// 检查底层 reader 是否实现了 io.Closer
	if closer, ok := fc.reader.(io.Closer); ok {
		err2 = closer.Close()
	}
	return errors.Join(err1, err2)
}
