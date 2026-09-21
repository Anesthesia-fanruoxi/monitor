package common

import (
	"bytes"
	"fmt"
	"io"
	"log"

	"github.com/klauspost/compress/gzip"
)

// MaxDecompressedSize 解压后大小限制（64MB）
// 10MB 的压缩数据可以膨胀到 GB 级，限制上限防止 gzip 炸弹打爆服务端内存
const MaxDecompressedSize = 64 * 1024 * 1024

// 用 github.com/klauspost/compress/gzip 替换标准库 compress/gzip
//
// klauspost/compress 是纯 Go 实现的高性能压缩库，兼容标准库 gzip 的 API 与
// 数据格式，但在多核场景下解压吞吐明显高于标准库。上报链路中"解压"是 CPU 热点，
// 切换后单请求的解压耗时下降，对高并发场景的 P99 有直接收益。
func Decompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := reader.Close(); err != nil {
			log.Printf("关闭 gzip reader 失败: %v", err)
		}
	}()

	// 读取解压后的内容（限制上限，防止压缩率极高的载荷撑爆内存）
	decompressed, err := io.ReadAll(io.LimitReader(reader, MaxDecompressedSize+1))
	if err != nil {
		return nil, err
	}
	if len(decompressed) > MaxDecompressedSize {
		return nil, fmt.Errorf("解压后数据超过 %d 字节上限", MaxDecompressedSize)
	}
	return decompressed, nil
}
