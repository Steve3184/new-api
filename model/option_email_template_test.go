package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionMapEmailVerificationTemplateUpdatesRuntimeTemplate(t *testing.T) {
	previousTemplate := common.EmailVerificationTemplate
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	previousOptionValue, hadPreviousOptionValue := common.OptionMap["EmailVerificationTemplate"]
	common.OptionMapRWMutex.Unlock()
	common.EmailVerificationTemplate = common.DefaultEmailVerificationTemplate
	t.Cleanup(func() {
		common.EmailVerificationTemplate = previousTemplate
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if hadPreviousOptionValue {
			common.OptionMap["EmailVerificationTemplate"] = previousOptionValue
			return
		}
		delete(common.OptionMap, "EmailVerificationTemplate")
	})

	template := `<p>Your code is {{.Code}}</p>`
	require.NoError(t, updateOptionMap("EmailVerificationTemplate", template))
	require.Equal(t, template, common.EmailVerificationTemplate)
	require.Equal(t, template, common.OptionMap["EmailVerificationTemplate"])
}
