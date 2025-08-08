package gateway

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
)

// JWTService handles JWT token operations
type JWTService struct {
	secretKey     []byte
	issuer        string
	tokenExpiry   time.Duration
	refreshExpiry time.Duration
}

// JWTClaims represents the claims in a JWT token
type JWTClaims struct {
	jwt.RegisteredClaims
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
}

// NewJWTService creates a new JWT service
func NewJWTService(secretKey string, issuer string, expiryHours int) *JWTService {
	return &JWTService{
		secretKey:     []byte(secretKey),
		issuer:        issuer,
		tokenExpiry:   time.Duration(expiryHours) * time.Hour,
		refreshExpiry: 7 * 24 * time.Hour, // Refresh tokens expire in 7 days (could be configurable later)
	}
}

// GenerateToken creates a new JWT token for the user
func (j *JWTService) GenerateToken(user *User) (string, error) {
	now := time.Now()
	claims := &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", user.ID), // Use subject claim with user ID
			Issuer:    j.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.tokenExpiry)),
			NotBefore: jwt.NewNumericDate(now),
		},
		UserID:   user.ID,
		Username: user.Username,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secretKey)
}

// ValidateToken validates a JWT token and returns the claims
func (j *JWTService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// PasswordService handles password hashing using Argon2
type PasswordService struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

// NewPasswordService creates a new password service with Argon2 settings
func NewPasswordService() *PasswordService {
	return &PasswordService{
		memory:      64 * 1024, // 64 MB
		iterations:  3,         // 3 iterations
		parallelism: 2,         // 2 threads
		saltLength:  16,        // 16 byte salt
		keyLength:   32,        // 32 byte key
	}
}

// HashPassword creates an Argon2 hash of the password
func (p *PasswordService) HashPassword(password string) (string, error) {
	// Generate a random salt
	salt := make([]byte, p.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Generate the hash
	hash := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, p.keyLength)

	// Encode the parameters and hash as a string
	// Format: $argon2id$v=19$m=65536,t=3,p=2$salt$hash
	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%x$%x",
		argon2.Version, p.memory, p.iterations, p.parallelism, salt, hash)

	return encoded, nil
}

// VerifyPassword verifies a password against an Argon2 hash
func (p *PasswordService) VerifyPassword(password, hashedPassword string) (bool, error) {
	// Parse the encoded hash
	memory, iterations, parallelism, salt, hash, err := p.parseHash(hashedPassword)
	if err != nil {
		return false, fmt.Errorf("failed to parse hash: %w", err)
	}

	// Hash the input password with the same parameters
	inputHash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, p.keyLength)

	// Compare the hashes
	if len(hash) != len(inputHash) {
		return false, nil
	}

	for i := 0; i < len(hash); i++ {
		if hash[i] != inputHash[i] {
			return false, nil
		}
	}

	return true, nil
}

// parseHash parses an encoded Argon2 hash string
func (p *PasswordService) parseHash(encodedHash string) (memory uint32, iterations uint32, parallelism uint8, salt, hash []byte, err error) {
	var version int
	n, err := fmt.Sscanf(encodedHash, "$argon2id$v=%d$m=%d,t=%d,p=%d$%x$%x",
		&version, &memory, &iterations, &parallelism, &salt, &hash)
	if err != nil || n != 6 {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid hash format")
	}

	if version != argon2.Version {
		return 0, 0, 0, nil, nil, fmt.Errorf("incompatible version")
	}

	return memory, iterations, parallelism, salt, hash, nil
}

// AuthMiddleware handles JWT authentication for protected routes
type AuthMiddleware struct {
	jwtService *JWTService
	database   *Database
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(jwtService *JWTService, database *Database) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService: jwtService,
		database:   database,
	}
}

// RequireAuth is a middleware that requires valid JWT authentication
func (a *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Check if header starts with "Bearer "
		const bearerPrefix = "Bearer "
		if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
			http.Error(w, "Authorization header must start with 'Bearer '", http.StatusUnauthorized)
			return
		}

		// Extract token
		tokenString := authHeader[len(bearerPrefix):]

		// Validate token
		claims, err := a.jwtService.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Optional: Verify user still exists in database
		user, err := a.database.GetUser(claims.UserID)
		if err != nil {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		// Add user to request context
		ctx := r.Context()
		ctx = context.WithValue(ctx, "user", user)
		ctx = context.WithValue(ctx, "claims", claims)

		// Continue with the request
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserFromContext extracts the authenticated user from the request context
func GetUserFromContext(r *http.Request) (*User, bool) {
	if user, ok := r.Context().Value("user").(*User); ok {
		return user, true
	}
	return nil, false
}