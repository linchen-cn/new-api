package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

const (
	SMSVerificationRateLimitMark = "SV"
	SMSVerificationMaxRequests   = 1  // 60秒内最多1次
	SMSVerificationDuration      = 60 // 60秒时间窗口
)

func redisSMSVerificationRateLimiter(c *gin.Context) {
	ctx := context.Background()
	rdb := common.RDB
	key := "smsVerification:" + SMSVerificationRateLimitMark + ":" + c.ClientIP()

	count, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		memorySMSVerificationRateLimiter(c)
		return
	}

	if count == 1 {
		_ = rdb.Expire(ctx, key, time.Duration(SMSVerificationDuration)*time.Second).Err()
	}

	if count <= int64(SMSVerificationMaxRequests) {
		c.Next()
		return
	}

	ttl, err := rdb.TTL(ctx, key).Result()
	waitSeconds := int64(SMSVerificationDuration)
	if err == nil && ttl > 0 {
		waitSeconds = int64(ttl.Seconds())
	}

	c.JSON(http.StatusTooManyRequests, gin.H{
		"success": false,
		"message": fmt.Sprintf("发送过于频繁，请等待 %d 秒后再试", waitSeconds),
	})
	c.Abort()
}

func memorySMSVerificationRateLimiter(c *gin.Context) {
	key := SMSVerificationRateLimitMark + ":" + c.ClientIP()

	if !inMemoryRateLimiter.Request(key, SMSVerificationMaxRequests, SMSVerificationDuration) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success": false,
			"message": "发送过于频繁，请稍后再试",
		})
		c.Abort()
		return
	}

	c.Next()
}

func SMSVerificationRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if common.RedisEnabled {
			redisSMSVerificationRateLimiter(c)
		} else {
			inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
			memorySMSVerificationRateLimiter(c)
		}
	}
}
