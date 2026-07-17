package config

import (
	"time"

	accesscfg "github.com/mondegor/go-core/mraccess/config"
	processcfg "github.com/mondegor/go-core/mrprocess/config"

	jwtcrypt "github.com/mondegor/go-components/mrauth/bag/jwt/crypt"
)

type (
	// UserRealm - конфигурация realm (области аутентификации): токены, виды пользователей, подтверждение операций.
	//
	// ОГРАНИЧЕНИЕ НА '/' В ИМЕНАХ. Name и UserKind.Name склеиваются в группу пользователя вида
	// "{Name}/{UserKind.Name}", а трассировщик активности разбирает её обратно, отрезая всё после
	// ПОСЛЕДНЕГО '/' (обе операции - в mrauth/model/usergroup).
	// Поэтому Name содержать '/' может (например "site/admin"), а UserKind.Name - нет.
	// Если '/' попадёт в UserKind.Name, realm определится неверно, не найдётся в реестре, и активность
	// пользователей этого вида пойдёт с сентинелом RealmID = 0: per-realm статистика потеряется
	// (в лог уйдёт лишь одно сообщение в час), хотя сессии и журнал сохранятся.
	// Ограничение проверяется один раз на старте хоста - ValidateRealms (fail-fast вместо тихой потери).
	UserRealm struct {
		ID uint16 `yaml:"id"` // числовой идентификатор realm (хранится в БД)

		// Name - имя realm: ключ реестра mrauth.RealmRegistry и граница системы (HTTP, token scopes,
		// отображение). Может содержать '/', см. ограничение выше.
		Name string `yaml:"name"`

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
		// Name - имя вида пользователя. НЕ должно содержать '/':
		// см. ограничение на '/' в описании UserRealm.
		Name string `yaml:"name"`

		Roles      []string `yaml:"roles"`
		SessionMax uint16   `yaml:"session_max"`
	}

	// OperationConfirm - настройки подтверждения операции: длина токена/кода, срок жизни, способы отправки.
	OperationConfirm struct {
		TokenLength     uint16        `yaml:"token_length"`
		CodeLength      uint8         `yaml:"code_length"`
		CodeMaxAttempts uint8         `yaml:"code_max_attempts"` // число попыток ввести код подтверждения операции
		SessionExpiry   time.Duration `yaml:"session_expiry"`
		SendByEmail     CodeSender    `yaml:"send_by_email"`
		SendByPhone     CodeSender    `yaml:"send_by_phone"`
	}

	// CodeSender - настройки отправки кода подтверждения: лимиты попыток, повторов и интервал между ними.
	CodeSender struct {
		MaxAttempts   uint8         `yaml:"max_attempts"`
		MaxResends    uint8         `yaml:"max_resends"`
		MinResendTime time.Duration `yaml:"min_resend_time"`
	}

	// Auth2FA - настройки второго фактора: аварийные (recovery) коды.
	Auth2FA struct {
		RecoveryCount        uint8 `yaml:"recovery_count"`         // число выдаваемых аварийных кодов
		RecoveryCodeLength   uint8 `yaml:"recovery_code_length"`   // длина одного аварийного кода
		RecoveryLowThreshold uint8 `yaml:"recovery_low_threshold"` // остаток, при котором слать предупреждение
	}

	// RefreshCookie - настройки cookie с refresh токеном (web-версия).
	RefreshCookie struct {
		Name     string        `yaml:"name"`
		Domain   string        `yaml:"domain" env:"APPX_REFRESH_COOKIE_DOMAIN"`
		Path     string        `yaml:"path"`
		Expiry   time.Duration `yaml:"expiry"`
		Secure   *bool         `yaml:"secure"`    // флаг Secure cookie; nil - безопасный дефолт (true)
		SameSite string        `yaml:"same_site"` // strict/lax/none; пусто - безопасный дефолт (strict); none требует secure=true
	}

	// AccessControl - корневая конфигурация аутентификации модуля: realm'ы, роли, токены и ключи JWT.
	AccessControl struct {
		Realms                  []UserRealm             `yaml:"realms"`
		ActionGroups            []accesscfg.ActionGroup `yaml:"action_groups"`
		RolesDirPath            string                  `yaml:"roles_dir_path" env:"APPX_ROLES_DIR_PATH" env-required:"true"`
		Roles                   []string                `yaml:"roles"`
		OverrideAuthToken       Token                   `yaml:"override_auth_token"`
		DefaultOperationConfirm OperationConfirm        `yaml:"default_operation_confirm"`
		Auth2FA                 Auth2FA                 `yaml:"auth_2fa"`

		// SessionSoftThreshold - отклонение от лимита сессий, при достижении которого пользователь
		// ставится в очередь на фоновую чистку.
		SessionSoftThreshold int8 `yaml:"session_soft_threshold"`

		// SessionHardThreshold - отклонение от лимита сессий, при достижении которого вход временно
		// отклоняется (должно быть не ниже soft).
		SessionHardThreshold int8 `yaml:"session_hard_threshold"`
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
	// Realm и Kind склеиваются в группу тем же usergroup.Build, что и у настоящего пользователя,
	// поэтому на них распространяется то же ограничение на '/', что описано у UserRealm:
	// Kind не должен его содержать. ValidateRealms этот отладочный конфиг не покрывает.
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
		OperationLogCollector    processcfg.MessageCollector `yaml:"operation_log_collector"`
	}
)
