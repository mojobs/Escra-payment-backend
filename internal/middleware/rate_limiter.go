package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	sredis "github.com/ulule/limiter/v3/drivers/store/redis"

	"github.com/mojobs/lara-payment-backend.git/internal/database"
	"github.com/mojobs/lara-payment-backend.git/internal/models"
)

func RateLimiterMiddleware() gin.HandlerFunc {
	store, err := sredis.NewStoreWithOptions(database.GetRedis(), limiter.StoreOptions{
		Prefix:   "rate_limiter",
		MaxRetry: 3,
	})

	if err != nil {
		panic(err)
	}

	rate := limiter.Rate{
		Period: 1 * time.Minute,
		Limit:  100,
	}
	instance := limiter.New(store, rate)
	middleware := mgin.NewMiddleware(instance)
	return func(c *gin.Context) {
		middleware(c)
	}
}
func StrictRateLimiterMiddleware() gin.HandlerFunc {
	store, err := sredis.NewStoreWithOptions(database.GetRedis(), limiter.StoreOptions{
		Prefix:   "strict_rate_limiter",
		MaxRetry: 3,
	})
	if err != nil {
		panic(err)
	}
	rate := limiter.Rate{
		Period: 1 * time.Minute,
		Limit:  10,
	}
	instance := limiter.New(store, rate)

	return func(c *gin.Context) {
		context := c.Request.Context()
		limiterCtx, err := instance.Get(context, c.ClientIP())

		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "rate_limiter_error",
				Message: "Rate limiter error ",
			})
			c.Abort()
			return
		}

		if limiterCtx.Reached {
			c.JSON(http.StatusTooManyRequests, models.ErrorResponse{
				Error:   "rate_limiter_exceeded",
				Message: "Too many requests. Please try again later",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func UserRateLimiterMiddleware() gin.HandlerFunc {
	store, err := sredis.NewStoreWithOptions(database.GetRedis(), limiter.StoreOptions{
		Prefix:   "user_rate_limiter",
		MaxRetry: 3,
	})
	if err != nil {
		panic(err)
	}

	rate := limiter.Rate{
		Period: 1 * time.Minute,
		Limit:  50,
	}

	instance := limiter.New(store, rate)

	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.Next()
			return
		}
		context := c.Request.Context()
		limiterCtx, err := instance.Get(context, userID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "rate_limiter_error",
				Message: "Rate limiter error",
			})

			c.Abort()
			return
		}

		if limiterCtx.Reached {
			c.JSON(http.StatusTooManyRequests, models.ErrorResponse{
				Error:   "rate_limit_exceeded",
				Message: "You've made too many requests. Please try again later.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
