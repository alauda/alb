/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"net/url"
	"os"
	"testing"

	f "alauda.io/alb2/test/e2e/framework"
	tu "alauda.io/alb2/utils/test_utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

func InitEnvTest() (cfg *rest.Config, base string, cancel func()) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv := &envtest.Environment{}
	if os.Getenv("DEV_MODE") == "true" {
		testEnv.ControlPlane.Etcd = &envtest.Etcd{
			URL: &url.URL{
				Scheme: "http",
				Host:   "127.0.0.1:32345",
			},
		}
	}
	// cfg is defined in this file globally.
	cfg, err := testEnv.Start()
	f.Logf("etcd %v", testEnv.ControlPlane.Etcd.URL)
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())
	base = tu.InitBase()
	return cfg, base, func() {
		testEnv.Stop()
	}
}
