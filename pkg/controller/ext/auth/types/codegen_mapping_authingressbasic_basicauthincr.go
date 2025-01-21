package types

import (
	"strings"
)

func init() {
	// make go happy
	_ = strings.Clone("")
}

type ReAssignAuthIngressBasicToBasicAuthInCrOpt struct{}

var ReAssignAuthIngressBasicToBasicAuthInCrTrans = map[string]func(lt *AuthIngressBasic, rt *BasicAuthInCr, opt *ReAssignAuthIngressBasicToBasicAuthInCrOpt) error{}

func ReAssignAuthIngressBasicToBasicAuthInCr(lt *AuthIngressBasic, rt *BasicAuthInCr, opt *ReAssignAuthIngressBasicToBasicAuthInCrOpt) error {
	if lt.AuthType != "" {
		rt.AuthType = lt.AuthType
	}

	if lt.Realm != "" {
		rt.Realm = lt.Realm
	}

	if lt.Secret != "" {
		rt.Secret = lt.Secret
	}

	if lt.SecretType != "" {
		rt.SecretType = lt.SecretType
	}

	for _, m := range ReAssignAuthIngressBasicToBasicAuthInCrTrans {
		err := m(lt, rt, opt)
		if err != nil {
			return err
		}
	}
	return nil
}
