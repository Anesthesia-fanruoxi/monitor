package store

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"monitor-server/modle"
)

var (
	// Prometheus 指标名允许 : 是为了兼容 recording rule 的命名习惯
	metricNameRe = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
	labelNameRe  = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

// validateSpec 校验动态指标描述的合法性
func validateSpec(spec DynamicMetricSpec) error {
	if spec.Name == "" {
		return errors.New("指标名不能为空")
	}
	if len(spec.Name) > MaxMetricNameLen {
		return fmt.Errorf("指标名长度超过 %d", MaxMetricNameLen)
	}
	if !metricNameRe.MatchString(spec.Name) {
		return fmt.Errorf("指标名 %q 含非法字符", spec.Name)
	}
	if len(spec.Help) > MaxMetricHelpLen {
		return fmt.Errorf("指标说明长度超过 %d", MaxMetricHelpLen)
	}
	if spec.Type != "" && spec.Type != modle.MetricTypeGauge {
		return fmt.Errorf("不支持的指标类型 %q（目前仅支持 %s）", spec.Type, modle.MetricTypeGauge)
	}
	if len(spec.Labels) > MaxLabelsPerMetric {
		return fmt.Errorf("标签数量超过 %d", MaxLabelsPerMetric)
	}
	if spec.Timeout > MaxTimeout {
		return fmt.Errorf("注销阈值超过上限 %s", MaxTimeout)
	}
	if spec.Timeout > 0 && spec.Timeout < MinTimeout {
		return fmt.Errorf("注销阈值不能小于 %s（否则时间序列会在两次采集之间被清掉）", MinTimeout)
	}

	seen := make(map[string]struct{}, len(spec.Labels))
	for _, name := range spec.Labels {
		if !labelNameRe.MatchString(name) {
			return fmt.Errorf("标签名 %q 含非法字符", name)
		}
		// __ 开头是 Prometheus 的保留前缀
		if strings.HasPrefix(name, "__") {
			return fmt.Errorf("标签名 %q 使用了 Prometheus 保留前缀 __", name)
		}
		if name == modle.ProjectLabel {
			return fmt.Errorf("标签 %s 由服务端统一注入，插件不得声明", modle.ProjectLabel)
		}
		if _, dup := seen[name]; dup {
			return fmt.Errorf("标签名 %q 重复声明", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}
