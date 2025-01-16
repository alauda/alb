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
	rt.AuthType = lt.AuthType
	rt.Realm = lt.Realm
	rt.Secret = lt.Secret
	rt.SecretType = lt.SecretType
	for _, m := range ReAssignAuthIngressBasicToBasicAuthInCrTrans {
		err := m(lt, rt, opt)
		if err != nil {
			return err
		}
	}
	return nil
}
