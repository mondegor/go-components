package config

import (
	"time"

	"github.com/mondegor/go-webcore/mraccess/config"
)

type (
	// UserRealm - comment struct.
	UserRealm struct {
		Name             string           `yaml:"name"`
		AuthToken        Token            `yaml:"auth_token"`
		UserKinds        []UserKind       `yaml:"user_kinds"`
		RegisterUserKind string           `yaml:"register_user_kind"`
		OperationConfirm OperationConfirm `yaml:"operation_confirm"`
	}

	// Token - comment struct.
	Token struct {
		AccessType    string        `yaml:"access_type"`
		AccessExpiry  time.Duration `yaml:"access_expiry"`
		RefreshExpiry time.Duration `yaml:"refresh_expiry"`
		Length        uint16        `yaml:"length"`
	}

	// UserKind - comment struct.
	UserKind struct {
		Name       string   `yaml:"name"`
		Roles      []string `yaml:"roles"`
		SessionMax uint32   `yaml:"session_max"`
	}

	// OperationConfirm - comment struct.
	OperationConfirm struct {
		TokenLength   uint16        `yaml:"token_length"`
		CodeLength    uint8         `yaml:"code_length"`
		SessionExpiry time.Duration `yaml:"session_expiry"`
		SendByEmail   CodeSender    `yaml:"send_by_email"`
		SendByPhone   CodeSender    `yaml:"send_by_phone"`
	}

	// CodeSender - comment struct.
	CodeSender struct {
		MaxAttempts   uint8         `yaml:"max_attempts"`
		MaxResends    uint8         `yaml:"max_resends"`
		MinResendTime time.Duration `yaml:"min_resend_time"`
	}

	// JWT - comment struct.
	JWT struct {
		Method string
		Secret []byte
	}

	// AccessControl - comment struct.
	AccessControl struct {
		Realms           []UserRealm          `yaml:"realms"`
		ActionGroups     []config.ActionGroup `yaml:"action_groups"`
		RolesDirPath     string               `yaml:"roles_dir_path" env:"APPX_ROLES_DIR_PATH"`
		Roles            []string             `yaml:"roles"`
		Privileges       []string             `yaml:"privileges"`
		Permissions      []string             `yaml:"permissions"`
		OperationConfirm OperationConfirm     `yaml:"operation_confirm"`
		JWTMethod        string               `yaml:"jwt_method" env:"APPX_JWT_METHOD"`
		JWTSecret        string               `yaml:"jwt_secret" env:"APPX_JWT_SECRET"`
	}

	// AuthorizedUser - comment struct.
	AuthorizedUser struct {
		ID       string `yaml:"id"`
		Realm    string `yaml:"realm"`
		Kind     string `yaml:"kind"`
		LangCode string `yaml:"lang"`
	}
)
