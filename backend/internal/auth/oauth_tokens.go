package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"
)

const (
	oauthProviderGoogle = "google"
	oauthProviderApple  = "apple"
)

type oauthIdentity struct {
	Provider           string
	Subject            string
	Email              string
	EmailAuthoritative bool
	FirstName          string
	LastName           string
	Name               string
	AvatarURL          string
}

type oauthProviderConfig struct {
	Provider string
	ClientID string
	Issuers  []string
	JWKSURL  string
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
}

type jwtClaims struct {
	Issuer        string `json:"iss"`
	Subject       string `json:"sub"`
	Audience      any    `json:"aud"`
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	HostedDomain  string `json:"hd"`
	ExpiresAt     int64  `json:"exp"`
	NotBefore     int64  `json:"nbf,omitempty"`
	IssuedAt      int64  `json:"iat,omitempty"`
}

type jwkSet struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	KeyType   string   `json:"kty"`
	KeyID     string   `json:"kid"`
	Algorithm string   `json:"alg"`
	Use       string   `json:"use"`
	Modulus   string   `json:"n"`
	Exponent  string   `json:"e"`
	Certs     []string `json:"x5c"`
}

func normalizeOAuthLoginInput(input OAuthLoginInput) (OAuthLoginInput, error) {
	normalized := OAuthLoginInput{
		Provider:  strings.ToLower(strings.TrimSpace(input.Provider)),
		IDToken:   strings.TrimSpace(input.IDToken),
		FirstName: sanitizeOAuthName(input.FirstName),
		LastName:  sanitizeOAuthName(input.LastName),
	}

	fieldErrors := make(map[string]string)
	if normalized.Provider != oauthProviderGoogle && normalized.Provider != oauthProviderApple {
		fieldErrors["provider"] = "Choose Google or Apple."
	}

	if normalized.IDToken == "" {
		fieldErrors["idToken"] = "Provider token is required."
	}

	if len(fieldErrors) > 0 {
		return OAuthLoginInput{}, &ValidationError{
			Message: "Please try the social sign-in again.",
			Fields:  fieldErrors,
		}
	}

	return normalized, nil
}

func (s Service) verifyOAuthIdentity(ctx context.Context, input OAuthLoginInput) (oauthIdentity, error) {
	providerConfig, err := s.oauthProviderConfig(input.Provider)
	if err != nil {
		return oauthIdentity{}, err
	}

	claims, err := verifyIDToken(ctx, providerConfig, input.IDToken)
	if err != nil {
		return oauthIdentity{}, err
	}

	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if email == "" {
		return oauthIdentity{}, ErrInvalidOAuthToken
	}

	if !claimIsTrue(claims.EmailVerified) {
		return oauthIdentity{}, ErrInvalidOAuthToken
	}

	identity := oauthIdentity{
		Provider:           providerConfig.Provider,
		Subject:            strings.TrimSpace(claims.Subject),
		Email:              email,
		EmailAuthoritative: providerConfig.Provider == oauthProviderApple || strings.HasSuffix(email, "@gmail.com") || strings.TrimSpace(claims.HostedDomain) != "",
		FirstName:          sanitizeOAuthName(claims.GivenName),
		LastName:           sanitizeOAuthName(claims.FamilyName),
		Name:               sanitizeOAuthName(claims.Name),
		AvatarURL:          sanitizeOAuthURL(claims.Picture),
	}

	if identity.Provider == oauthProviderApple {
		if identity.FirstName == "" {
			identity.FirstName = input.FirstName
		}

		if identity.LastName == "" {
			identity.LastName = input.LastName
		}
	}

	if identity.Subject == "" {
		return oauthIdentity{}, ErrInvalidOAuthToken
	}

	return identity, nil
}

func (s Service) oauthProviderConfig(provider string) (oauthProviderConfig, error) {
	switch provider {
	case oauthProviderGoogle:
		clientID := strings.TrimSpace(s.oauth.GoogleClientID)
		if clientID == "" {
			return oauthProviderConfig{}, ErrOAuthNotConfigured
		}

		return oauthProviderConfig{
			Provider: oauthProviderGoogle,
			ClientID: clientID,
			Issuers:  []string{"https://accounts.google.com", "accounts.google.com"},
			JWKSURL:  "https://www.googleapis.com/oauth2/v3/certs",
		}, nil
	case oauthProviderApple:
		clientID := strings.TrimSpace(s.oauth.AppleClientID)
		if clientID == "" {
			return oauthProviderConfig{}, ErrOAuthNotConfigured
		}

		return oauthProviderConfig{
			Provider: oauthProviderApple,
			ClientID: clientID,
			Issuers:  []string{"https://appleid.apple.com"},
			JWKSURL:  "https://appleid.apple.com/auth/keys",
		}, nil
	default:
		return oauthProviderConfig{}, ErrInvalidOAuthToken
	}
}

func verifyIDToken(ctx context.Context, provider oauthProviderConfig, rawToken string) (jwtClaims, error) {
	header, claims, signingInput, signature, err := parseJWT(rawToken)
	if err != nil {
		return jwtClaims{}, ErrInvalidOAuthToken
	}

	if header.Algorithm != "RS256" || header.KeyID == "" {
		return jwtClaims{}, ErrInvalidOAuthToken
	}

	publicKey, err := fetchPublicKey(ctx, provider.JWKSURL, header.KeyID)
	if err != nil {
		return jwtClaims{}, err
	}

	digest := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return jwtClaims{}, ErrInvalidOAuthToken
	}

	now := time.Now().UTC().Unix()
	if claims.ExpiresAt <= now {
		return jwtClaims{}, ErrInvalidOAuthToken
	}

	if claims.NotBefore > 0 && claims.NotBefore > now+60 {
		return jwtClaims{}, ErrInvalidOAuthToken
	}

	if !stringInList(claims.Issuer, provider.Issuers) {
		return jwtClaims{}, ErrInvalidOAuthToken
	}

	if !audienceMatches(claims.Audience, provider.ClientID) {
		return jwtClaims{}, ErrInvalidOAuthToken
	}

	if strings.TrimSpace(claims.Subject) == "" {
		return jwtClaims{}, ErrInvalidOAuthToken
	}

	return claims, nil
}

func parseJWT(rawToken string) (jwtHeader, jwtClaims, string, []byte, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return jwtHeader{}, jwtClaims{}, "", nil, fmt.Errorf("jwt must have three parts")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return jwtHeader{}, jwtClaims{}, "", nil, err
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return jwtHeader{}, jwtClaims{}, "", nil, err
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return jwtHeader{}, jwtClaims{}, "", nil, err
	}

	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return jwtHeader{}, jwtClaims{}, "", nil, err
	}

	var claims jwtClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return jwtHeader{}, jwtClaims{}, "", nil, err
	}

	return header, claims, strings.Join(parts[:2], "."), signature, nil
}

func fetchPublicKey(ctx context.Context, jwksURL, keyID string) (*rsa.PublicKey, error) {
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build jwks request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch jwks: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read jwks: %w", err)
	}

	var set jwkSet
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, fmt.Errorf("decode jwks: %w", err)
	}

	for _, key := range set.Keys {
		if key.KeyID != keyID {
			continue
		}

		publicKey, err := key.rsaPublicKey()
		if err != nil {
			return nil, err
		}

		return publicKey, nil
	}

	return nil, ErrInvalidOAuthToken
}

func (k jwkKey) rsaPublicKey() (*rsa.PublicKey, error) {
	if len(k.Certs) > 0 {
		certBytes, err := base64.StdEncoding.DecodeString(k.Certs[0])
		if err != nil {
			return nil, ErrInvalidOAuthToken
		}

		cert, err := x509.ParseCertificate(certBytes)
		if err != nil {
			return nil, ErrInvalidOAuthToken
		}

		publicKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, ErrInvalidOAuthToken
		}

		return publicKey, nil
	}

	if k.KeyType != "RSA" || k.Modulus == "" || k.Exponent == "" {
		return nil, ErrInvalidOAuthToken
	}

	modulusBytes, err := base64.RawURLEncoding.DecodeString(k.Modulus)
	if err != nil {
		return nil, ErrInvalidOAuthToken
	}

	exponentBytes, err := base64.RawURLEncoding.DecodeString(k.Exponent)
	if err != nil {
		return nil, ErrInvalidOAuthToken
	}

	exponent := 0
	for _, b := range exponentBytes {
		exponent = exponent<<8 + int(b)
	}

	if exponent == 0 {
		return nil, ErrInvalidOAuthToken
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(modulusBytes),
		E: exponent,
	}, nil
}

func (i oauthIdentity) displayNames() (string, string) {
	firstName := sanitizeOAuthName(i.FirstName)
	lastName := sanitizeOAuthName(i.LastName)

	if firstName == "" || lastName == "" {
		parts := strings.Fields(sanitizeOAuthName(i.Name))
		if firstName == "" && len(parts) > 0 {
			firstName = parts[0]
		}

		if lastName == "" && len(parts) > 1 {
			lastName = strings.Join(parts[1:], " ")
		}
	}

	if firstName == "" {
		firstName = sanitizeOAuthName(strings.Split(i.Email, "@")[0])
	}

	if firstName == "" {
		firstName = providerDisplayName(i.Provider)
	}

	if lastName == "" {
		lastName = "Account"
	}

	return firstName, lastName
}

func providerDisplayName(provider string) string {
	switch provider {
	case oauthProviderGoogle:
		return "Google"
	case oauthProviderApple:
		return "Apple"
	default:
		return "Social"
	}
}

func sanitizeOAuthName(value string) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	runes := []rune(normalized)
	if len(runes) > 120 {
		return string(runes[:120])
	}

	return normalized
}

func sanitizeOAuthURL(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return ""
	}

	if strings.HasPrefix(strings.ToLower(normalized), "http://") || strings.HasPrefix(strings.ToLower(normalized), "https://") {
		return normalized
	}

	return ""
}

func claimIsTrue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func stringInList(value string, items []string) bool {
	for _, item := range items {
		if value == item {
			return true
		}
	}

	return false
}

func audienceMatches(value any, expected string) bool {
	switch typed := value.(type) {
	case string:
		return typed == expected
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok && text == expected {
				return true
			}
		}
	default:
		return false
	}

	return false
}
