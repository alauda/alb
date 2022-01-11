package test_utils

import (
	"fmt"
	"os"
	"os/exec"
)

// GenCert will generate a certificate use given domain.
// you need to install openssl first.
func GenCert(domain string) (key, cert string, err error) {
	dir, err := os.MkdirTemp("", "cert")
	if err != nil {
		return "", "", err
	}
	keyPath := fmt.Sprintf("%s/key", dir)
	certPath := fmt.Sprintf("%s/cert", dir)
	shell := fmt.Sprintf(`openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes  -keyout %s -out %s -subj  /CN="%s"`, keyPath, certPath, domain)
	_, err = exec.Command("bash", "-c", shell).CombinedOutput()
	if err != nil {
		return "", "", err
	}
	keyByte, err := os.ReadFile(keyPath)
	if err != nil {
		return "", "", err
	}
	certByte, err := os.ReadFile(certPath)
	if err != nil {
		return "", "", err
	}
	os.RemoveAll(dir)
	return string(keyByte), string(certByte), nil
}
