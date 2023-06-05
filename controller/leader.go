package controller

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"alauda.io/alb2/config"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coordinationv1client "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

type LeaderElection struct {
	lock     sync.RWMutex // we need access isLeader under different goroutine,so protect with a lock
	isLeader bool
	// leader atomic.B
	ctx context.Context
	c   config.IConfig
	cfg *rest.Config
	log logr.Logger
}

func NewLeaderElection(ctx context.Context, c config.IConfig, cfg *rest.Config, log logr.Logger) *LeaderElection {
	return &LeaderElection{
		ctx: ctx,
		c:   c,
		cfg: cfg,
		log: log.WithValues("pod", c.GetPodName()),
	}
}

func (l *LeaderElection) AmILeader() bool {
	l.lock.RLock()
	defer l.lock.RUnlock()
	return l.isLeader
}

func (l *LeaderElection) becomeLeader() {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.isLeader = true
}

func (l *LeaderElection) looseLeader() {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.isLeader = false
}

// start leader election process,it should be run in goroutine.
// TODO add wait group to each xx loop
func (l *LeaderElection) StartLeaderElectionLoop() error {
	log := l.log
	c := l.c
	cfg := l.cfg
	ctx := l.ctx

	lcfg := c.GetLeaderConfig()
	leaseLockName := c.GetAlbName()
	leaseLockNamespace := c.GetNs()
	podname := c.GetPodName()
	id := fmt.Sprintf("%s/%s/%s", leaseLockNamespace, leaseLockName, podname)

	log.Info("leader election thread start", "cfg", lcfg, "id", id, "alb", leaseLockName)

	client, err := coordinationv1client.NewForConfig(cfg)
	if err != nil {
		return err
	}
	// we use the Lease lock type since edits to Leases are less common
	// and fewer objects in the cluster watch "all Leases".
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      leaseLockName,
			Namespace: leaseLockNamespace,
		},
		Client: client,
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}
	// start the leader election code loop
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock: lock,
		// IMPORTANT: you MUST ensure that any code you have that
		// is protected by the lease must terminate **before**
		// you call cancel. Otherwise, you could have a background
		// loop still running and another process could
		// get elected before your background loop finished, violating
		// the stated goal of the lease.
		ReleaseOnCancel: true,
		LeaseDuration:   lcfg.LeaseDuration,
		RenewDeadline:   lcfg.RenewDeadline,
		RetryPeriod:     lcfg.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				// we're notified when we start - this is where you would
				// usually put your code
				log.Info("leader start", "id", id)
			},
			OnStoppedLeading: func() {
				l.looseLeader()
				// we can do cleanup here
				if errors.Is(ctx.Err(), context.Canceled) {
					log.Info("ctx canceled", "id", id)
				} else {
					log.Error(nil, "leader lost", "id", id)
					// IMPORTANT lossing leader means sth is wrong. let it just die.
					// TODO we need a event system.
					os.Exit(-1)
				}
			},
			OnNewLeader: func(identity string) {
				// we're notified when new leader elected
				if identity == id {
					l.becomeLeader()
					// I just got the lock
					log.Info("new leader is me", "my-id", identity)
					return
				}
				log.Info("new leader elected", "leader-id", identity, "my-id", id)
			},
		},
	})

	if errors.Is(ctx.Err(), context.Canceled) {
		log.Info("ctx canceled. leader election stop")
		return nil
	}
	err = fmt.Errorf("impossiable,out of leader loop?")
	log.Error(err, "id", id)
	return err
}

func (l *LeaderElection) WaitUtilIMLeader() {
	// OPTIMIZE use sth like event(channel) base mechanism
	for {
		if l.AmILeader() {
			return
		}
		time.Sleep(time.Second * 1)
	}
}
