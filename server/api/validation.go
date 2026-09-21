package api

import (
	"fmt"
	"log"
	"sync"
	"time"
	"unicode/utf8"
)

// ===== 上报基础校验与限额常量 =====

// MaxRequestBodySize 请求体大小限制（10MB）
const MaxRequestBodySize = 10 * 1024 * 1024

// MaxLabelValueLen 单个标签值（字符串字段）允许的最大字节数
const MaxLabelValueLen = 512

// MaxClockSkew 允许的上报时间戳与服务器时间的最大偏差
const MaxClockSkew = 5 * time.Minute

var missingTimestampWarnOnce sync.Once

// validateTimestamp 校验上报时间戳（毫秒），用于抵御密文重放
func validateTimestamp(timestampMs int64) error {
	if timestampMs == 0 {
		missingTimestampWarnOnce.Do(func() {
			log.Printf("上报数据未携带 timestamp 字段，无法做重放校验（建议升级上报端）")
		})
		return nil
	}
	if timestampMs <= 0 {
		return fmt.Errorf("timestamp 字段格式错误")
	}

	skew := time.Since(time.UnixMilli(timestampMs))
	if skew < 0 {
		skew = -skew
	}
	if skew > MaxClockSkew {
		return fmt.Errorf("上报时间戳与当前时间相差 %s，超出允许范围", skew.Truncate(time.Second))
	}
	return nil
}

// isValidProject 验证 project 名称是否合法
// 限制长度（按 rune 计算，支持中文）；ASCII 场景下 fast path 避免每次都转 []rune
func isValidProject(project string) bool {
	if len(project) == 0 || len(project) > 256 {
		return false
	}
	if len(project) <= 64 {
		return isValidProjectChars(project)
	}
	if utf8.RuneCountInString(project) > 64 {
		return false
	}
	return isValidProjectChars(project)
}

func isValidProjectChars(project string) bool {
	for _, c := range project {
		isAlphaNum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		isSpecialChar := c == '_' || c == '-' || c == '.'
		isChinese := c >= 0x4E00 && c <= 0x9FFF
		if !isAlphaNum && !isSpecialChar && !isChinese {
			return false
		}
	}
	return true
}
