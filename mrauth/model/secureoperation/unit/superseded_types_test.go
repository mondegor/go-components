package unit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mondegor/go-components/mrauth/enum/operationtype"
	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

func TestSupersededTypes(t *testing.T) {
	t.Parallel()

	changeEmailChain := []operationtype.Enum{operationtype.ChangeEmail, operationtype.ChangeEmailConfirm}

	tests := []struct {
		opType operationtype.Enum
		want   []operationtype.Enum
	}{
		{opType: operationtype.ChangeEmail, want: changeEmailChain},
		{opType: operationtype.ChangeEmailConfirm, want: changeEmailChain},
		{opType: operationtype.ChangePhone, want: []operationtype.Enum{operationtype.ChangePhone}},
		{opType: operationtype.Disable2FA, want: []operationtype.Enum{operationtype.Disable2FA}},
	}

	for _, tt := range tests {
		t.Run(tt.opType.String(), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, unit.SupersededTypes(tt.opType))
		})
	}
}
