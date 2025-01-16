package auth

import (
	"fmt"
	"strings"

	ct "alauda.io/alb2/controller/types"
	. "alauda.io/alb2/pkg/controller/ext/auth/types"
	"github.com/go-logr/logr"

	. "alauda.io/alb2/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

type BasicAuthCtl struct {
	l logr.Logger
}

func (f BasicAuthCtl) AuthIngressToAuthCr(auth_ingress *AuthIngress, auth_cr *AuthCr) {
	auth_cr.Basic = &BasicAuthInCr{
		Realm:      "",
		Secret:     "",
		SecretType: "",
		AuthType:   "",
	}
	_ = ReassignStructViaMapping(auth_ingress, auth_cr.Basic, ReassignStructOpt{})
}

func (b BasicAuthCtl) ToPolicy(basic *BasicAuthInCr, p *AuthPolicy, refs ct.RefMap, rule string) {
	log := b.l.WithValues("rule", rule)
	bp := &BasicAuthPolicy{
		Realm:    "",
		Secret:   map[string]BasicAuthHash{},
		AuthType: "",
		Err:      "",
	}
	p.Basic = bp
	_ = ReassignStructViaMapping(basic, bp, ReassignStructOpt{})
	if bp.AuthType != "basic" {
		bp.Err = "only support basic auth"
		return
	}

	key, err := ParseStringToObjectKey(basic.Secret)
	if err != nil {
		log.Error(err, "invalid secret refs", "key", key)
		bp.Err = "invalid secret refs format"
		return
	}
	if secret := refs.Secret[key]; secret == nil {
		log.Error(err, "secret refs ", key)
		bp.Err = "secret refs not found"
		return
	}
	bp.Secret, err = parseSecret(refs.Secret[key], basic.SecretType)
	if err != nil {
		bp.Err = "invalid secret context " + err.Error()
	}
}

func parseHash(hash string, name string) (BasicAuthHash, error) {
	cfg := BasicAuthHash{}
	if !strings.Contains(hash, "$apr1$") {
		return cfg, fmt.Errorf("unsupported algorithm")
	}
	parts := strings.Split(hash, "$apr1$")
	if len(parts) != 2 {
		return cfg, fmt.Errorf("invalid pass format")
	}
	if name == "" {
		name_in_hash, has_suffix := strings.CutSuffix(parts[0], ":")
		if !has_suffix {
			return cfg, fmt.Errorf("invalid pass format")
		}
		name = name_in_hash
	}
	pass := strings.Split(parts[1], "$")
	if len(parts) != 2 {
		return cfg, fmt.Errorf("invalid pass format")
	}
	cfg.Algorithm = "apr1"
	cfg.Name = name
	cfg.Salt = pass[0]
	cfg.Hash = pass[1]
	return cfg, nil
}

func parseSecret(secret *corev1.Secret, secret_type string) (map[string]BasicAuthHash, error) {
	ret := map[string]BasicAuthHash{}
	if secret_type == "auth-file" {
		hash_cfg, err := parseHash(string(secret.Data["auth"]), "")
		if err != nil {
			return nil, err
		}
		ret[hash_cfg.Name] = hash_cfg
		return ret, nil
	}

	if secret_type == "auth-map" {
		for name, hash := range secret.Data {
			hash_cfg, err := parseHash(string(hash), name)
			if err != nil {
				return nil, err
			}
			ret[hash_cfg.Name] = hash_cfg
		}
	}
	return ret, nil
}
