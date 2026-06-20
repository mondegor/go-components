package config

import (
	"fmt"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/bag/jwt/crypt"
)

const (
	jwtAlgHS256 = "HS256"
	jwtAlgHS512 = "HS512"
	jwtAlgRS256 = "RS256"
	jwtAlgES256 = "ES256"
)

// InitJWT - строит ключи подписи (issuer) и проверки (verifier) JWT из конфигурации.
// Вызывается, когда хотя бы один realm использует access_type=jwt (см. IsJWTUsed);
// ключевой материал (cfg.Secret) обязателен. Если ключи отсутствуют или некорректны,
// возвращает ошибку (инвариант проверяется через validateJWT).
func InitJWT(cfg JWT) (JWT, error) {
	// гарантируем, что для realm'ов с jwt ключевой материал задан и корректен, иначе verifier был бы nil
	if err := validateJWT(cfg); err != nil {
		return JWT{}, err
	}

	var signingKey crypt.SigningKey

	switch cfg.Alg {
	case jwtAlgHS256, jwtAlgHS512:
		key, err := crypt.NewHMACKey(cfg.KID, cfg.Alg, []byte(cfg.Secret))
		if err != nil {
			return JWT{}, fmt.Errorf("HMAC signing key: %w", err)
		}

		signingKey = key
	case jwtAlgRS256:
		key, err := crypt.NewRSAKeyFromPEM(cfg.KID, []byte(cfg.Secret))
		if err != nil {
			return JWT{}, fmt.Errorf("RSA signing key: %w", err)
		}

		signingKey = key
	case jwtAlgES256:
		key, err := crypt.NewECDSAKeyFromPEM(cfg.KID, []byte(cfg.Secret))
		if err != nil {
			return JWT{}, fmt.Errorf("ECDSA signing key: %w", err)
		}

		signingKey = key
	default:
		return JWT{}, fmt.Errorf("unsupported JWT alg: %q", cfg.Alg)
	}

	verifyKeys := make([]crypt.Key, 0, 1+len(cfg.VerifyKeys))
	verifyKeys = append(verifyKeys, signingKey)

	for _, vk := range cfg.VerifyKeys {
		key, err := newVerifyKey(vk)
		if err != nil {
			return JWT{}, err
		}

		verifyKeys = append(verifyKeys, key)
	}

	verifier, err := crypt.NewKeySet(verifyKeys...)
	if err != nil {
		return JWT{}, err
	}

	cfg.SigningKey = signingKey
	cfg.Verifier = verifier

	return cfg, nil
}

// newVerifyKey - строит ключ только для проверки подписи из публичного PEM.
func newVerifyKey(vk JWTVerifyKey) (crypt.Key, error) {
	switch vk.Alg {
	case jwtAlgRS256:
		key, err := crypt.NewRSAVerifyKeyFromPEM(vk.KID, []byte(vk.PublicKey))
		if err != nil {
			return nil, fmt.Errorf("RSA verify key (kid=%q): %w", vk.KID, err)
		}

		return key, nil
	case jwtAlgES256:
		key, err := crypt.NewECDSAVerifyKeyFromPEM(vk.KID, []byte(vk.PublicKey))
		if err != nil {
			return nil, fmt.Errorf("ECDSA verify key (kid=%q): %w", vk.KID, err)
		}

		return key, nil
	default:
		return nil, fmt.Errorf("unsupported verify key alg: %q (kid=%q)", vk.Alg, vk.KID)
	}
}

// validateJWT - проверяет наличие ключевого материала JWT, если хотя бы один realm использует jwt.
func validateJWT(cfg JWT) error {
	switch cfg.Alg {
	case jwtAlgHS256, jwtAlgHS512, jwtAlgRS256, jwtAlgES256:
	default:
		return fmt.Errorf("unsupported JWT alg: %q", cfg.Alg)
	}

	if cfg.Secret == "" {
		return errors.New("JWT secret (HMAC secret or PEM private key) is required")
	}

	// слабый HMAC-секрет brute-force'ится и позволяет подделывать токены - требуем минимум по RFC 7518
	switch cfg.Alg {
	case jwtAlgHS256:
		if len(cfg.Secret) < minHMACSecretHS256 {
			return fmt.Errorf("HMAC secret for %q must be at least %d bytes", jwtAlgHS256, minHMACSecretHS256)
		}
	case jwtAlgHS512:
		if len(cfg.Secret) < minHMACSecretHS512 {
			return fmt.Errorf("HMAC secret for %q must be at least %d bytes", jwtAlgHS512, minHMACSecretHS512)
		}
	}

	// kid обязателен у активного ключа: токены всегда несут 'kid', ротация бесшовная
	if cfg.KID == "" {
		return errors.New("JWT kid is required")
	}

	return validateJWTVerifyKeys(cfg)
}

// validateJWTVerifyKeys - проверяет ключи проверки подписи (период ротации): kid обязателен
// и уникален среди всех ключей набора.
func validateJWTVerifyKeys(cfg JWT) error {
	seenKIDs := map[string]bool{cfg.KID: true}

	for i, vk := range cfg.VerifyKeys {
		if vk.KID == "" {
			return fmt.Errorf("JWT verify key kid is required (index=%d)", i)
		}

		if seenKIDs[vk.KID] {
			return fmt.Errorf("duplicate JWT key kid %q (verify key index=%d)", vk.KID, i)
		}

		seenKIDs[vk.KID] = true

		switch vk.Alg {
		case jwtAlgRS256, jwtAlgES256:
		default:
			return fmt.Errorf("unsupported JWT verify key alg %q (kid=%q)", vk.Alg, vk.KID)
		}

		if vk.PublicKey == "" {
			return fmt.Errorf("JWT verify key public_key is required (kid=%q)", vk.KID)
		}
	}

	return nil
}
