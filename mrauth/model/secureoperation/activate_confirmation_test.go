package secureoperation_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

func TestSecureOperation_NextAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		token         string
		operationType operationtype.Enum
		actions       []secureoperation.ConfirmAction
	}{
		{
			name:          "test1",
			token:         "token",
			operationType: operationtype.ChangePhone,
			actions: []secureoperation.ConfirmAction{
				{
					Method:        confirmmethod.Email,
					CodeLength:    6,
					MaxAttempts:   10,
					MaxResends:    5,
					MinResendTime: 5 * time.Minute,
					Expiry:        10 * time.Minute,
					Address:       "",
					ConfirmCode:   "secret1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			op, err := secureoperation.NewOperation(tt.token, tt.operationType, uuid.Nil, tt.actions, nil)
			require.NoError(t, err)

			err = op.ActivateConfirmation("secret1")
			require.NoError(t, err)
		})
	}
}
