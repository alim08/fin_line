package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alim08/fin_line/pkg/logger"
	"github.com/alim08/fin_line/pkg/metrics"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// Claims represents JWT claims
type Claims struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// AuthService handles JWT authentication
type AuthService struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	issuer     string
	audience   string
	expiration time.Duration
}

// Config holds authentication configuration
type Config struct {
	PrivateKeyPath string
	PublicKeyPath  string
	Issuer         string
	Audience       string
	Expiration     time.Duration
}

// NewConfig creates a new auth configuration from environment variables
func NewConfig() *Config {
	return &Config{
		PrivateKeyPath: getEnvOrDefault("JWT_PRIVATE_KEY_PATH", "keys/private.pem"),
		PublicKeyPath:  getEnvOrDefault("JWT_PUBLIC_KEY_PATH", "keys/public.pem"),
		Issuer:         getEnvOrDefault("JWT_ISSUER", "fin-line"),
		Audience:       getEnvOrDefault("JWT_AUDIENCE", "fin-line-api"),
		Expiration:     getEnvDurationOrDefault("JWT_EXPIRATION", 24*time.Hour),
	}
}

// NewAuthService creates a new authentication service
func NewAuthService(config *Config) (*AuthService, error) {
	// Load private key
	privateKey, err := loadPrivateKey(config.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load private key: %w", err)
	}

	// Load public key
	publicKey, err := loadPublicKey(config.PublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load public key: %w", err)
	}

	return &AuthService{
		privateKey: privateKey,
		publicKey:  publicKey,
		issuer:     config.Issuer,
		audience:   config.Audience,
		expiration: config.Expiration,
	}, nil
}

// GenerateToken generates a new JWT token for a user
func (a *AuthService) GenerateToken(userID, username, email string, roles []string) (string, error) {
	start := time.Now()
	defer func() {
		metrics.AuthOperationDuration.WithLabelValues("generate_token", "success").Observe(time.Since(start).Seconds())
	}()

	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Email:    email,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    a.issuer,
			Audience:  []string{a.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.expiration)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(a.privateKey)
	if err != nil {
		metrics.AuthOperationDuration.WithLabelValues("generate_token", "error").Observe(time.Since(start).Seconds())
		metrics.AuthErrors.WithLabelValues("generate_token").Inc()
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	metrics.AuthOperations.WithLabelValues("generate_token", "success").Inc()
	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
func (a *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	start := time.Now()
	defer func() {
		metrics.AuthOperationDuration.WithLabelValues("validate_token", "success").Observe(time.Since(start).Seconds())
	}()

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.publicKey, nil
	})

	if err != nil {
		metrics.AuthOperationDuration.WithLabelValues("validate_token", "error").Observe(time.Since(start).Seconds())
		metrics.AuthErrors.WithLabelValues("validate_token").Inc()
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		metrics.AuthOperationDuration.WithLabelValues("validate_token", "invalid").Observe(time.Since(start).Seconds())
		metrics.AuthErrors.WithLabelValues("validate_token").Inc()
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		metrics.AuthOperationDuration.WithLabelValues("validate_token", "invalid_claims").Observe(time.Since(start).Seconds())
		metrics.AuthErrors.WithLabelValues("validate_token").Inc()
		return nil, fmt.Errorf("invalid token claims")
	}

	// Validate issuer and audience
	if claims.Issuer != a.issuer {
		return nil, fmt.Errorf("invalid issuer")
	}

	audienceValid := false
	for _, aud := range claims.Audience {
		if aud == a.audience {
			audienceValid = true
			break
		}
	}
	if !audienceValid {
		return nil, fmt.Errorf("invalid audience")
	}

	metrics.AuthOperations.WithLabelValues("validate_token", "success").Inc()
	return claims, nil
}

// HasRole checks if the user has a specific role
func (c *Claims) HasRole(role string) bool {
	for _, userRole := range c.Roles {
		if userRole == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the user has any of the specified roles
func (c *Claims) HasAnyRole(roles ...string) bool {
	for _, requiredRole := range roles {
		if c.HasRole(requiredRole) {
			return true
		}
	}
	return false
}

// HasAllRoles checks if the user has all of the specified roles
func (c *Claims) HasAllRoles(roles ...string) bool {
	for _, requiredRole := range roles {
		if !c.HasRole(requiredRole) {
			return false
		}
	}
	return true
}

// AuthMiddleware creates middleware for JWT authentication
func (a *AuthService) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			metrics.AuthMiddlewareDuration.Observe(time.Since(start).Seconds())
		}()

		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			metrics.AuthMiddlewareErrors.WithLabelValues("missing_header").Inc()
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Check Bearer token format
		if !strings.HasPrefix(authHeader, "Bearer ") {
			metrics.AuthMiddlewareErrors.WithLabelValues("invalid_format").Inc()
			http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Validate token
		claims, err := a.ValidateToken(tokenString)
		if err != nil {
			logger.Log.Warn("token validation failed", zap.Error(err), zap.String("ip", r.RemoteAddr))
			metrics.AuthMiddlewareErrors.WithLabelValues("invalid_token").Inc()
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Add claims to request context
		ctx := context.WithValue(r.Context(), "user", claims)
		next.ServeHTTP(w, r.WithContext(ctx))

		metrics.AuthMiddlewareSuccess.Inc()
	})
}

// RoleMiddleware creates middleware for role-based access control
func (a *AuthService) RoleMiddleware(requiredRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			defer func() {
				metrics.AuthMiddlewareDuration.Observe(time.Since(start).Seconds())
			}()

			// Get user from context
			user, ok := r.Context().Value("user").(*Claims)
			if !ok {
				metrics.AuthMiddlewareErrors.WithLabelValues("no_user_context").Inc()
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			// Check if user has required roles
			if !user.HasAnyRole(requiredRoles...) {
				logger.Log.Warn("insufficient permissions", 
					zap.String("user_id", user.UserID),
					zap.Strings("user_roles", user.Roles),
					zap.Strings("required_roles", requiredRoles))
				metrics.AuthMiddlewareErrors.WithLabelValues("insufficient_permissions").Inc()
				http.Error(w, "Insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
			metrics.AuthMiddlewareSuccess.Inc()
		})
	}
}

// GetUserFromContext extracts user claims from context
func GetUserFromContext(ctx context.Context) (*Claims, bool) {
	user, ok := ctx.Value("user").(*Claims)
	return user, ok
}

// GenerateKeyPair generates a new RSA key pair for JWT signing
func GenerateKeyPair(bits int) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	publicKey := &privateKey.PublicKey
	return privateKey, publicKey, nil
}

// SavePrivateKey saves a private key to PEM format
func SavePrivateKey(privateKey *rsa.PrivateKey, filename string) error {
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	return writeFile(filename, privateKeyPEM)
}

// SavePublicKey saves a public key to PEM format
func SavePublicKey(publicKey *rsa.PublicKey, filename string) error {
	publicKeyBytes := x509.MarshalPKCS1PublicKey(publicKey)
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return writeFile(filename, publicKeyPEM)
}

// loadPrivateKey loads a private key from PEM file
func loadPrivateKey(filename string) (*rsa.PrivateKey, error) {
	data, err := readFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return privateKey, nil
}

// loadPublicKey loads a public key from PEM file
func loadPublicKey(filename string) (*rsa.PublicKey, error) {
	data, err := readFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	publicKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	return publicKey, nil
}

// Helper functions for environment variable parsing
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// File operations
func readFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

func writeFile(filename string, data []byte) error {
	// Ensure directory exists
	dir := getDir(filename)
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}
	return os.WriteFile(filename, data, 0600)
}

func getDir(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '/' || filename[i] == '\\' {
			return filename[:i]
		}
	}
	return ""
} 