package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var ErrInvalidToken = errors.New("invalid token")

type Role string

const (
	RoleAdmin      Role = "admin"
	RoleEditor     Role = "editor"
	RoleReadOnly   Role = "readonly"
	RoleReportView Role = "reportview"
)

// Rank is used for RequireRole(min). reportview is 0 so it never satisfies
// RoleReadOnly+ IPAM routes; use CanViewReports for report endpoints.
func (r Role) Rank() int {
	switch r {
	case RoleAdmin:
		return 3
	case RoleEditor:
		return 2
	case RoleReadOnly:
		return 1
	case RoleReportView:
		return 0
	default:
		return 0
	}
}

func (r Role) CanViewReports() bool {
	return r == RoleReportView || r.Rank() >= RoleReadOnly.Rank()
}

func (r Role) IsReportOnly() bool {
	return r == RoleReportView
}

func ParseRole(s string) (Role, error) {
	switch Role(s) {
	case RoleAdmin, RoleEditor, RoleReadOnly, RoleReportView:
		return Role(s), nil
	default:
		return "", fmt.Errorf("invalid role: %s", s)
	}
}

type Claims struct {
	UserID     uuid.UUID `json:"uid"`
	Email      string    `json:"email"`
	Role       Role      `json:"role"`
	MFAPending bool      `json:"mfa,omitempty"`
	IsAPIToken bool      `json:"-"`
	TokenID    uuid.UUID `json:"-"`
	TokenName  string    `json:"-"`
	jwt.RegisteredClaims
}

type TokenService struct {
	secret []byte
	ttl    time.Duration
}

const mfaPendingTTL = 5 * time.Minute

func NewTokenService(secret string, ttl time.Duration) *TokenService {
	return &TokenService{secret: []byte(secret), ttl: ttl}
}

func (s *TokenService) Issue(userID uuid.UUID, email string, role Role) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// IssueMFAPending issues a short-lived token used only for the MFA login challenge.
func (s *TokenService) IssueMFAPending(userID uuid.UUID, email string, role Role) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:     userID,
		Email:      email,
		Role:       role,
		MFAPending: true,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(mfaPendingTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *TokenService) Parse(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.secret, nil
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

// ParseSession rejects MFA-pending tokens so they cannot access the API.
func (s *TokenService) ParseSession(tokenStr string) (*Claims, error) {
	claims, err := s.Parse(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.MFAPending {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
