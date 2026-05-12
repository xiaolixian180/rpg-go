// Package utils 提供游戏服务器中常用的通用工具函数。
// 包含随机字符串生成、数值范围限制等基础能力，
// 避免在业务代码中重复实现这些通用逻辑。
package utils

import "math/rand"

// charset 是生成随机字符串时使用的字符集，包含大小写字母和数字，
// 共 62 个字符，确保生成的字符串具有良好的可读性和唯一性。
const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// RandString 生成指定长度的随机字符串。
// 从 charset 中随机选取字符拼接而成，适用于生成临时令牌、
// 随机 ID、验证码等不需要密码学安全保证的场景。
// 注意：使用 math/rand 而非 crypto/rand，不适用于安全敏感的密钥生成。
func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		// 从 charset 中随机选取一个字符
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// Clamp 将整数值 v 限制在 [min, max] 区间内。
// 若 v 小于 min 则返回 min，若 v 大于 max 则返回 max，否则返回 v 本身。
// 常用于限制玩家属性值、伤害数值等不超过合法范围。
func Clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ClampFloat 将浮点数值 v 限制在 [min, max] 区间内。
// 逻辑与 Clamp 相同，但针对 float64 类型，
// 常用于限制概率值（如暴击率 0.0~1.0）、伤害倍率等浮点参数。
func ClampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
