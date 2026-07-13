package secureoperation_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/mondegor/go-components/mrauth/enum/confirmmethod"
	"github.com/mondegor/go-components/mrauth/enum/operationstatus"
	"github.com/mondegor/go-components/mrauth/model/secureoperation"
)

func Test_NewOperationWithError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		token          string
		operationName  string
		actions        []secureoperation.ConfirmAction
		wantErrMessage string
	}{
		{
			name:           "test1",
			wantErrMessage: "name is empty",
		},
		{
			name:           "test2",
			operationName:  "name1",
			wantErrMessage: "operation is opened, but len(actions) == 0",
		},
		{
			name:          "test3",
			operationName: "name1",
			actions: []secureoperation.ConfirmAction{
				{
					Method: 0,
				},
			},
			wantErrMessage: "action without method",
		},
		{
			name:          "test4",
			operationName: "name1",
			actions: []secureoperation.ConfirmAction{
				{
					Method: confirmmethod.Email,
				},
			},
			wantErrMessage: "token is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := secureoperation.NewOperation(tt.token, tt.operationName, uuid.Nil, tt.actions, nil)
			assert.ErrorContains(t, err, tt.wantErrMessage)
		})
	}
}

func Test_WakeUpOperationWithError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		token          string
		operationName  string
		actions        []secureoperation.ConfirmAction
		status         operationstatus.Enum
		wantErrMessage string
	}{
		{
			name:           "test1",
			wantErrMessage: "token is empty",
		},
		{
			name:           "test2",
			token:          "token",
			wantErrMessage: "name is empty",
		},
		{
			name:           "test3",
			token:          "token",
			operationName:  "name1",
			wantErrMessage: "operation status is unknown",
		},
		{
			name:          "test4",
			token:         "token",
			operationName: "name1",
			actions: []secureoperation.ConfirmAction{
				{
					Method: 0,
				},
			},
			status:         operationstatus.Opened,
			wantErrMessage: "action without method",
		},
		{
			name:          "test5",
			token:         "token",
			operationName: "name1",
			actions: []secureoperation.ConfirmAction{
				{},
			},
			status:         operationstatus.Confirmed,
			wantErrMessage: "operation is confirmed, but len(actions) > 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			op := secureoperation.SecureOperation{
				Token:             tt.token,
				Name:              tt.operationName,
				UserID:            uuid.Nil,
				RemainingAttempts: 0,
				RemainingResends:  0,
				ResendsAt:         time.Time{},
				Payload:           nil,
				Status:            tt.status,
				ExpiresAt:         time.Time{},
			}

			assert.ErrorContains(t, secureoperation.WakeUp(&op, tt.actions), tt.wantErrMessage)
		})
	}
}
