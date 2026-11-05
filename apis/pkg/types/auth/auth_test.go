package auth

import (
	"testing"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	"github.com/stretchr/testify/assert"
)

func TestConvertToErrCodeSeparatesAuthenticationFromAuthorization(t *testing.T) {
	assert.Equal(t, apimodel.Code_TokenNotExisted, ConvertToErrCode(ErrorTokenNotExist))
	assert.Equal(t, apimodel.Code_TokenNotExisted, ConvertToErrCode(ErrorTokenInvalid))
	assert.Equal(t, apimodel.Code_TokenDisabled, ConvertToErrCode(ErrorTokenDisabled))
	assert.Equal(t, apimodel.Code_NotAllowedAccess, ConvertToErrCode(ErrorNoUser))
}
