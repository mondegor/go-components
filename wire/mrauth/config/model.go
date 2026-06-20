package config

import (
	"time"

	accesscfg "github.com/mondegor/go-sysmess/mraccess/config"
	processcfg "github.com/mondegor/go-sysmess/mrprocess/config"

	jwtcrypt "github.com/mondegor/go-components/mrauth/bag/jwt/crypt"
)

type (
	// UserRealm - конфигурация realm (области аутентификации): токены, виды пользователей, подтверждение операций.
	UserRealm struct {
		Name             string           `yaml:"name"`
		AuthToken        Token            `yaml:"auth_token"`
		UserKinds        []UserKind       `yaml:"user_kinds"`
		RegisterUserKind string           `yaml:"register_user_kind"`
		OperationConfirm OperationConfirm `yaml:"operation_confirm"`
	}

	// Token - настройки access/refresh токенов realm (тип доступа, сроки жизни, длина).
	Token struct {
		AccessType    string        `yaml:"access_type"`
		AccessExpiry  time.Duration `yaml:"access_expiry"`
		RefreshExpiry time.Duration `yaml:"refresh_expiry"`
		Length        uint16        `yaml:"length"` // for refresh and access[type == 'session']
	}

	// UserKind - вид пользователя внутри realm: набор ролей и максимум одновременных сессий.
	UserKind struct {
		Name       string   `yaml:"name"`
		Roles      []string `yaml:"roles"`
		SessionMax uint32   `yaml:"session_max"`
	}

	// OperationConfirm - настройки подтверждения операции: длина токена/кода, срок жизни, способы отправки.
	OperationConfirm struct {
		TokenLength   uint16        `yaml:"token_length"`
		CodeLength    uint8         `yaml:"code_length"`
		SessionExpiry time.Duration `yaml:"session_expiry"`
		SendByEmail   CodeSender    `yaml:"send_by_email"`
		SendByPhone   CodeSender    `yaml:"send_by_phone"`
	}

	// CodeSender - настройки отправки кода подтверждения: лимиты попыток, повторов и интервал между ними.
	CodeSender struct {
		MaxAttempts   uint8         `yaml:"max_attempts"`
		MaxResends    uint8         `yaml:"max_resends"`
		MinResendTime time.Duration `yaml:"min_resend_time"`
	}

	// RefreshCookie - настройки cookie с refresh токеном (web-версия).
	RefreshCookie struct {
		Name   string        `yaml:"name"`
		Domain string        `yaml:"domain" env:"APPX_REFRESH_COOKIE_DOMAIN"`
		Path   string        `yaml:"path"`
		Expiry time.Duration `yaml:"expiry"`
	}

	// AccessControl - корневая конфигурация аутентификации модуля: realm'ы, роли, токены и ключи JWT.
	AccessControl struct {
		Realms                  []UserRealm             `yaml:"realms"`
		ActionGroups            []accesscfg.ActionGroup `yaml:"action_groups"`
		RolesDirPath            string                  `yaml:"roles_dir_path" env:"APPX_ROLES_DIR_PATH" env-required:"true"`
		Roles                   []string                `yaml:"roles"`
		OverrideAuthToken       Token                   `yaml:"override_auth_token"`
		DefaultOperationConfirm OperationConfirm        `yaml:"default_operation_confirm"`
	}

	// JWT - настройки и ключи для подписи (issuer) и проверки (verifier) access-токенов.
	JWT struct {
		Issuer string `yaml:"issuer" env:"APPX_JWT_ISSUER"` // издатель токена (claim iss)
		Alg    string `yaml:"alg" env:"APPX_JWT_ALG"`       // алгоритм подписи: HS256/HS512/RS256/ES256
		KID    string `yaml:"kid" env:"APPX_JWT_KID"`       // идентификатор активного ключа подписи (обязателен для jwt)

		// Secret - HMAC-секрет (HS256/HS512) либо приватный ключ в формате PEM (RS256/ES256).
		Secret string `yaml:"secret" env:"APPX_JWT_SECRET"`

		// VerifyKeys - дополнительные ключи только для проверки подписи (старые ключи в период ротации, RS256/ES256).
		VerifyKeys []JWTVerifyKey `yaml:"verify_keys"`

		SigningKey jwtcrypt.SigningKey // активный ключ подписи
		Verifier   jwtcrypt.KeySet     // набор ключей проверки подписи
	}

	// JWTVerifyKey - асимметричный ключ только для проверки подписи (публичная часть в PEM).
	JWTVerifyKey struct {
		KID       string `yaml:"kid"`
		Alg       string `yaml:"alg"` // RS256/ES256
		PublicKey string `yaml:"public_key"`
	}

	// TestUser - тестовый пользователь для отладки: при заданном ID подменяет реальную аутентификацию в своём realm.
	TestUser struct {
		ID       string
		Realm    string
		Kind     string
		LangCode string
	}

	// TaskSchedule - настройки задач модуля Auth, запускаемых по расписанию.
	TaskSchedule struct {
		// Caption           string        `yaml:"caption"`
		CleanRecords             processcfg.SchedulerTask    `yaml:"clean_records"`
		CleanRecordsLimit        uint32                      `yaml:"clean_records_limit"`
		LogsLifeTime             time.Duration               `yaml:"logs_life_time"`
		UserStatRequestCollector processcfg.MessageCollector `yaml:"user_stat_request_collector"`
	}
)
