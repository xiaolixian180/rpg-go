package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 自定义 JWT 声明，携带玩家ID和标准注册声明
type Claims struct {
	PlayerID uint64 `json:"player_id"`
	jwt.RegisteredClaims
}

// JWTManager JWT 令牌管理器，负责令牌的签发与解析
type JWTManager struct {
	secretKey []byte
	expireDur time.Duration
}

// NewJWTManager 创建 JWT 管理器实例。
// secretKey 用于 HMAC 签名，expireHours 为令牌有效时长（小时）。
func NewJWTManager(secretKey string, expireHours int) *JWTManager {
	return &JWTManager{
		secretKey: []byte(secretKey),
		expireDur: time.Duration(expireHours) * time.Hour,
	}
}

// GenerateToken 为指定玩家签发 JWT 令牌
func (m *JWTManager) GenerateToken(playerID uint64) (string, error) {
	claims := Claims{
		PlayerID: playerID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.expireDur)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

// ParseToken 解析并验证 JWT 令牌，返回自定义声明
func (m *JWTManager) ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secretKey, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
