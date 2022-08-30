package ingress

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// we should wait a interval
	t.Logf("start %v", time.Now())
	go wait.UntilWithContext(ctx, func(ctx context.Context) {
		t.Logf("in %v", time.Now())
	}, time.Millisecond*100)
	time.Sleep(time.Second * 1)
	cancel()
	t.Logf("end %v", time.Now())
	time.Sleep(time.Millisecond * 500)
}

func TestIngress(t *testing.T) {
	t.Logf("ok")
	RegisterFailHandler(Fail)
	RunSpecs(t, "ingress election ")
}

var _ = Describe("LeaderElection", func() {
	// TODO here
	It("should remove extra rule", func() {
		// 1. create ingress
		// 2. generate corresponse rule # TODO test point
		// 3. compare need update  # TODO test point actions crate/update/remove
		// 4. change ingress
		// 5. generate corresponse rule # TODO test point
		// 6. compare need update  # TODO test point actions crate/update/remove
	})
})
