package loop

import (
	"context"

	"github.com/qdm12/deunhealth/internal/docker"
	"github.com/qdm12/golibs/logging"
)

func newUnhealthyLoop(docker docker.Dockerer, logger logging.Logger) *unhealthyLoop {
	return &unhealthyLoop{
		docker: docker,
		logger: logger,
	}
}

type unhealthyLoop struct {
	logger logging.Logger
	docker docker.Dockerer
}

func (l *unhealthyLoop) Run(ctx context.Context) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	onUnhealthyNames, err := l.docker.GetLabeled(ctx, []string{"deunhealth.restart.on.unhealthy=true"})
	if err != nil {
		return err
	}
	l.logger.Info("Monitoring containers " + buildEnum(onUnhealthyNames) + " to restart when becoming unhealthy")

	l.logger.Info("New labeled containers will be automatically detected")

	existingUnhealthies, err := l.docker.GetUnhealthy(ctx)
	if err != nil {
		return err
	}
	for _, unhealthy := range existingUnhealthies {
		l.restartUnhealthy(ctx, unhealthy)
	}

	unhealthies := make(chan docker.Container)
	unhealthyStreamCrashed := make(chan error)

	go l.docker.StreamUnhealthy(ctx, unhealthies, unhealthyStreamCrashed)

	for {
		select {
		case <-ctx.Done():
			<-unhealthyStreamCrashed
			close(unhealthyStreamCrashed)
			close(unhealthies)

			return ctx.Err()

		case err := <-unhealthyStreamCrashed:
			close(unhealthyStreamCrashed)
			close(unhealthies)

			return err

		case unhealthy := <-unhealthies:
			l.restartUnhealthy(ctx, unhealthy)
		}
	}
}

func (l *unhealthyLoop) restartUnhealthy(ctx context.Context, unhealthy docker.Container) {
	l.logger.Info("container " + unhealthy.Name +
		" (image " + unhealthy.Image + ") is unhealthy, restarting it...")
	err := l.docker.RestartContainer(ctx, unhealthy.Name)
	if err != nil {
		l.logger.Error("failed restarting container: " + err.Error())
	} else {
		l.logger.Info("container " + unhealthy.Name + " restarted successfully")
	}
}