package middleware

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"ledger-api/app/internal/auth"
)

type supabaseClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type jwks struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Alg string `json:"alg"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func fetchECPublicKey(jwksURL string) (*ecdsa.PublicKey, error) {
	resp, err := http.Get(jwksURL) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	var keys jwks
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return nil, fmt.Errorf("decode JWKS: %w", err)
	}

	for _, k := range keys.Keys {
		if k.Alg == "ES256" || k.Crv == "P-256" {
			xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
			if err != nil {
				return nil, fmt.Errorf("decode x: %w", err)
			}
			yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
			if err != nil {
				return nil, fmt.Errorf("decode y: %w", err)
			}
			return &ecdsa.PublicKey{
				Curve: elliptic.P256(),
				X:     new(big.Int).SetBytes(xBytes),
				Y:     new(big.Int).SetBytes(yBytes),
			}, nil
		}
	}

	return nil, fmt.Errorf("no ES256 key found in JWKS")
}

// Auth validates Supabase JWTs using ES256 (asymmetric, verified via JWKS).
func Auth(jwksURL string) gin.HandlerFunc {
	var ecKey *ecdsa.PublicKey
	if jwksURL != "" {
		if key, err := fetchECPublicKey(jwksURL); err == nil {
			ecKey = key
		}
	}

	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims := &supabaseClaims{}

		_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			switch t.Method.(type) {
			case *jwt.SigningMethodECDSA:
				if ecKey == nil {
					return nil, fmt.Errorf("no EC key available for ES256")
				}
				return ecKey, nil
			default:
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
		})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		userID, err := claims.GetSubject()
		if err != nil || userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing subject claim"})
			return
		}

		ctx := auth.WithUser(c.Request.Context(), tokenStr, userID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
