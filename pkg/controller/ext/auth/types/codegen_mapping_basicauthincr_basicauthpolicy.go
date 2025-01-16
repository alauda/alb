package types

import (
	"strings"
)

func init() {
	// make go happy
	_ = strings.Clone("")
}

type ReAssignBasicAuthInCrToBasicAuthPolicyOpt struct{}

var ReAssignBasicAuthInCrToBasicAuthPolicyTrans = map[string]func(lt *BasicAuthInCr, rt *BasicAuthPolicy, opt *ReAssignBasicAuthInCrToBasicAuthPolicyOpt) error{}

func ReAssignBasicAuthInCrToBasicAuthPolicy(lt *BasicAuthInCr, rt *BasicAuthPolicy, opt *ReAssignBasicAuthInCrToBasicAuthPolicyOpt) error {
	rt.AuthType = lt.AuthType
	rt.Realm = lt.Realm
	for _, m := range ReAssignBasicAuthInCrToBasicAuthPolicyTrans {
		err := m(lt, rt, opt)
		if err != nil {
			return err
		}
	}
	return nil
}
