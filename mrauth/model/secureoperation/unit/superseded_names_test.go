package unit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mondegor/go-components/mrauth/model/secureoperation/unit"
)

func TestSupersededNames(t *testing.T) {
	t.Parallel()

	changeEmailChain := []string{unit.NameConfirmChangeEmailRequest, unit.NameConfirmChangeEmail}

	tests := []struct {
		name string
		want []string
	}{
		{name: unit.NameConfirmChangeEmailRequest, want: changeEmailChain},
		{name: unit.NameConfirmChangeEmail, want: changeEmailChain},
		{name: unit.NameConfirmChangePhone, want: []string{unit.NameConfirmChangePhone}},
		{name: unit.NameConfirmDisable2FA, want: []string{unit.NameConfirmDisable2FA}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, unit.SupersededNames(tt.name))
		})
	}
}
