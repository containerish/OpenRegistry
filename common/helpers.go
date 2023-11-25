package common

import (
	"github.com/containerish/OpenRegistry/types"
)

func RegistryErrorResponse(code, msg string, detail map[string]interface{}) types.RegistryErrors {
	return types.RegistryErrors{
		Errors: []types.RegistryError{
			{
				Code:    code,
				Message: msg,
				Detail:  detail,
			},
		},
	}
}
