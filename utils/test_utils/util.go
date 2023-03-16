package test_utils

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"path"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kcapi "k8s.io/client-go/tools/clientcmd/api"
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

func KubeConfigFromREST(cfg *rest.Config, envtestName string) ([]byte, error) {
	kubeConfig := kcapi.NewConfig()
	protocol := "https"
	if !rest.IsConfigTransportTLS(*cfg) {
		protocol = "http"
	}

	// cfg.Host is a URL, so we need to parse it so we can properly append the API path
	baseURL, err := url.Parse(cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("unable to interpret config's host value as a URL: %w", err)
	}

	kubeConfig.Clusters[envtestName] = &kcapi.Cluster{
		// TODO(directxman12): if client-go ever decides to expose defaultServerUrlFor(config),
		// we can just use that.  Note that this is not the same as the public DefaultServerURL,
		// which requires us to pass a bunch of stuff in manually.
		Server:                   (&url.URL{Scheme: protocol, Host: baseURL.Host, Path: cfg.APIPath}).String(),
		CertificateAuthorityData: cfg.CAData,
	}
	kubeConfig.AuthInfos[envtestName] = &kcapi.AuthInfo{
		// try to cover all auth strategies that aren't plugins
		ClientCertificateData: cfg.CertData,
		ClientKeyData:         cfg.KeyData,
		Token:                 cfg.BearerToken,
		Username:              cfg.Username,
		Password:              cfg.Password,
	}
	kcCtx := kcapi.NewContext()
	kcCtx.Cluster = envtestName
	kcCtx.AuthInfo = envtestName
	kubeConfig.Contexts[envtestName] = kcCtx
	kubeConfig.CurrentContext = envtestName

	contents, err := clientcmd.Write(*kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to serialize kubeconfig file: %w", err)
	}
	return contents, nil
}

func RandomFile(base string, file string) (string, error) {
	p := path.Join(base, fmt.Sprintf("%v", rand.Int()))
	err := os.WriteFile(p, []byte(file), os.ModePerm)
	return p, err
}

func PrettyJson(data interface{}) string {
	out, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return fmt.Sprintf("err: %v could not jsonlize %v", err, data)
	}
	return string(out)
}
