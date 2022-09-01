package ingress

import (
	"context"
	"time"

	"alauda.io/alb2/config"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *Controller) StartResyncLoop(ctx context.Context) error {
	log := c.log.WithName("resync")
	resyncPeriod := time.Duration(config.GetInt("RESYNC_PERIOD")) * time.Second

	if !config.GetBool("FULL_SYNC") {
		log.Info("periodicity sync disabled ingnore")
		return nil
	}
	log.Info("start periodicity sync", "period", resyncPeriod)
	// UntilWithContext will run immediately, we donot want resync and start ingressloop in the same time, so wait a resyncperiod
	log.Info("sleep first", "period", resyncPeriod)
	time.Sleep(resyncPeriod)
	wait.UntilWithContext(ctx, func(ctx context.Context) {
		err := c.OnResync(ctx, log)
		if err != nil {
			log.Error(err, "resync fail,just retry in next period")
		}
	}, resyncPeriod)
	return nil
}

func (c *Controller) OnResync(ctx context.Context, log logr.Logger) error {
	log.Info("doing a periodicity sync")
	// findHandledIngress
	alb, err := c.kd.LoadALB(config.GetAlbKey(c))
	if err != nil {
		log.Error(err, "load alb fail")
	}
	handledIngressKey := alb.FindHandledIngressKey()
	shouldHandledIngress := []*networkingv1.Ingress{}

	overHandledIngresskey := []client.ObjectKey{}
	unSyncdIngress := []client.ObjectKey{}

	allIngress, err := c.kd.ListAllIngress()
	if err != nil {
		log.Error(err, "list ingress fail")
	}

	for _, ing := range allIngress {
		key := IngKey(ing)
		should, _ := c.shouldHandleIngress(alb, ing)
		if should {
			shouldHandledIngress = append(shouldHandledIngress, ing)
			// needHandledIngress = append(needHandledIngress, ing)
			expect, err := c.generateExpect(alb, ing)
			if err != nil {
				return err
			}
			need, err := c.doUpdate(ing, alb, expect, true)
			if err != nil {
				return err
			}
			if need {
				log.Info("find a unsynced ingress", "key", ing)
				unSyncdIngress = append(unSyncdIngress, key)
			}
		}
	}
	shouldHandledIngressMap := lo.KeyBy(shouldHandledIngress, IngKey)
	for _, ing := range handledIngressKey {
		if _, exist := shouldHandledIngressMap[ing]; !exist {
			log.Info("find a over handled ingress", "key", ing)
			overHandledIngresskey = append(overHandledIngresskey, ing)
		}
	}
	log.Info("resync count over", "over-handle-ing-len", len(overHandledIngresskey), "unsyncd-ing-len", len(unSyncdIngress))
	keys := lo.KeyBy(append(unSyncdIngress, overHandledIngresskey...), func(key client.ObjectKey) client.ObjectKey {
		return key
	})
	for _, ingkey := range keys {
		c.enqueue(ingkey)
	}
	return nil
}
