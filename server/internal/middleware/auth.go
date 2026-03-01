package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Claims is the JWT payload we store in every access token.
type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// Auth returns a Gin middleware that validates a Bearer JWT and injects the
// caller's user ID into the request context under the key "userID".
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "authorization header is required",
				"data":    nil,
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "authorization header format must be 'Bearer <token>'",
				"data":    nil,
			})
			return
		}

		tokenStr := parts[1]

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})

		// jwt.ParseWithClaims validates the signature AND all RegisteredClaims
		// including ExpiresAt. An expired token causes err != nil (specifically
		// jwt.ErrTokenExpired) so the !token.Valid guard below correctly rejects
		// expired tokens without any additional expiry check needed.
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "invalid or expired token",
				"data":    nil,
			})
			return
		}

		if claims.UserID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "token missing user_id claim",
				"data":    nil,
			})
			return
		}

		// Inject the caller's user ID so handlers can use it without re-parsing.
		c.Set("userID", claims.UserID)
		c.Next()
	}
}
