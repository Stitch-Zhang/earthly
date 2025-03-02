package containerutil

import (
	"context"
	"fmt"
	"io"
	"runtime"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/earthly/earthly/conslogging"
)

// ContainerFrontend is an interface specifying all the container options Earthly needs to do.
type ContainerFrontend interface {
	Scheme() string

	IsAvaliable(ctx context.Context) bool
	Config() *CurrentFrontend
	Information(ctx context.Context) (*FrontendInfo, error)

	ContainerInfo(ctx context.Context, namesOrIDs ...string) (map[string]*ContainerInfo, error)
	ContainerRemove(ctx context.Context, force bool, namesOrIDs ...string) error
	ContainerStop(ctx context.Context, timeoutSec uint, namesOrIDs ...string) error
	ContainerLogs(ctx context.Context, namesOrIDs ...string) (map[string]*ContainerLogs, error)
	ContainerRun(ctx context.Context, containers ...ContainerRun) error

	ImageInfo(ctx context.Context, refs ...string) (map[string]*ImageInfo, error)
	ImagePull(ctx context.Context, refs ...string) error
	ImageRemove(ctx context.Context, force bool, refs ...string) error
	ImageTag(ctx context.Context, tags ...ImageTag) error
	ImageLoad(ctx context.Context, image ...io.Reader) error
	ImageLoadFromFileCommand(filename string) string

	VolumeInfo(ctx context.Context, volumeNames ...string) (map[string]*VolumeInfo, error)
}

// FrontendConfig is the configuration needed to bring up a given frontend. Includes logging and needed information to
// calculate URLs to reach the container.
type FrontendConfig struct {
	BuildkitHostCLIValue  string
	BuildkitHostFileValue string

	DebuggerHostCLIValue  string
	DebuggerHostFileValue string
	DebuggerPortFileValue int

	LocalRegistryHostFileValue string

	Console conslogging.ConsoleLogger
}

// FrontendForSetting returns a frontend given a setting. This includes automatic detection.
func FrontendForSetting(ctx context.Context, feType string, cfg *FrontendConfig) (ContainerFrontend, error) {
	if feType == FrontendAuto {
		return autodetectFrontend(ctx, cfg)
	}

	return frontendIfAvaliable(ctx, feType, cfg)
}

func autodetectFrontend(ctx context.Context, cfg *FrontendConfig) (ContainerFrontend, error) {
	var errs error

	for _, feType := range []string{
		FrontendDockerShell,
		FrontendPodmanShell,
	} {
		fe, err := frontendIfAvaliable(ctx, feType, cfg)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		return fe, nil
	}
	return nil, errors.Wrapf(errs, "failed to autodetect a supported frontend")
}

func frontendIfAvaliable(ctx context.Context, feType string, cfg *FrontendConfig) (ContainerFrontend, error) {
	var newFe func(context.Context, *FrontendConfig) (ContainerFrontend, error)
	switch feType {
	case FrontendDockerShell:
		newFe = NewDockerShellFrontend
	case FrontendPodmanShell:
		newFe = NewPodmanShellFrontend
	default:
		return nil, fmt.Errorf("%s is not a supported container frontend", feType)
	}

	fe, err := newFe(ctx, cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "%s frontend failed to initalize", feType)
	}
	if !fe.IsAvaliable(ctx) {
		return nil, fmt.Errorf("%s frontend not avaliable", feType)
	}

	return fe, nil
}

func getPlatform() string {
	arch := runtime.GOARCH
	if runtime.GOARCH == "arm" {
		arch = "arm/v7"
	}
	return fmt.Sprintf("linux/%s", arch)
}
