package services

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type Service interface {
	Start() error
	Stop() error
	Name() string
}

var execIDKey = struct{}{}

func ExecID(ctx context.Context) string {
	execID := ctx.Value(execIDKey)
	if execID == nil {
		panic("cannot get exec ID")
	}
	return execID.(string)
}

func initExecID(ctx context.Context) context.Context {
	execID, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}
	return context.WithValue(ctx, execIDKey, execID.String())
}

func InitMainContext() (context.Context, context.CancelFunc) {
	execIDCtx := initExecID(context.Background())
	ctx, cancel := context.WithCancel(execIDCtx)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		<-sigc
		cancel()
	}()
	return ctx, cancel
}

// StartServices kicks off all services and blocks until an error is returned or context ends
func StartServices(ctx context.Context, services []Service) error {
	grp, ctx := errgroup.WithContext(ctx)

	for _, service := range services {
		grp.Go(service.Start)
	}

	// wait for context to stop (service.Start may either block or be async)
	grp.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	// clean up all services
	defer func() {
		for _, service := range services {
			if err := service.Stop(); err != nil {
				log.Errorf("error stopping %s: %s", service.Name(), err.Error())
			}
		}
	}()

	if err := grp.Wait(); err != nil && err != context.Canceled {
		log.Errorf("Error returned from grp: %s", err.Error())
		return err
	}
	return nil
}