package component

import (
	"os"
	"strings"

	"github.com/redhat-developer/odo/pkg/devfile"

	"github.com/pkg/errors"
	"github.com/redhat-developer/odo/pkg/envinfo"
	"github.com/redhat-developer/odo/pkg/machineoutput"
	"github.com/redhat-developer/odo/pkg/odo/genericclioptions"
	"github.com/redhat-developer/odo/pkg/util"

	componentlabels "github.com/redhat-developer/odo/pkg/component/labels"
	"github.com/redhat-developer/odo/pkg/devfile/adapters"
	"github.com/redhat-developer/odo/pkg/devfile/adapters/common"
	"github.com/redhat-developer/odo/pkg/devfile/adapters/kubernetes"
	"github.com/redhat-developer/odo/pkg/log"
)

// DevfilePush has the logic to perform the required actions for a given devfile
func (po *PushOptions) DevfilePush() error {

	// Wrap the push so that we can capture the error in JSON-only mode
	err := po.devfilePushInner()

	if err != nil && log.IsJSON() {
		eventLoggingClient := machineoutput.NewConsoleMachineEventLoggingClient()
		eventLoggingClient.ReportError(err, machineoutput.TimestampNow())

		// Suppress the error to prevent it from being output by the generic machine-readable handler (which will produce invalid JSON for our purposes)
		err = nil

		// os.Exit(1) since we are suppressing the generic machine-readable handler's exit code logic
		os.Exit(1)
	}

	if err != nil {
		return err
	}

	// push is successful, save the runMode used
	runMode := envinfo.Run
	if po.debugFlag {
		runMode = envinfo.Debug
	}

	return po.EnvSpecificInfo.SetRunMode(runMode)
}

func (po *PushOptions) devfilePushInner() (err error) {
	devObj, err := devfile.ParseAndValidateFromFile(po.DevfilePath)
	if err != nil {
		return err
	}
	componentName := po.EnvSpecificInfo.GetName()

	// Set the source path to either the context or current working directory (if context not set)
	po.sourcePath, err = util.GetAbsPath(po.componentContext)
	if err != nil {
		return errors.Wrap(err, "unable to get source path")
	}

	// Apply ignore information
	err = genericclioptions.ApplyIgnore(&po.ignoreFlag, po.sourcePath)
	if err != nil {
		return errors.Wrap(err, "unable to apply ignore information")
	}

	var platformContext interface{}
	kc := kubernetes.KubernetesContext{
		Namespace: po.KClient.GetCurrentNamespace(),
	}
	platformContext = kc

	devfileHandler, err := adapters.NewComponentAdapter(componentName, po.sourcePath, po.GetApplication(), devObj, platformContext)
	if err != nil {
		return err
	}

	pushParams := common.PushParameters{
		Path:            po.sourcePath,
		IgnoredFiles:    po.ignoreFlag,
		ForceBuild:      po.forceBuildFlag,
		Show:            po.showFlag,
		EnvSpecificInfo: *po.EnvSpecificInfo,
		DevfileBuildCmd: strings.ToLower(po.buildCommandFlag),
		DevfileRunCmd:   strings.ToLower(po.runCommandflag),
		DevfileDebugCmd: strings.ToLower(po.debugCommandFlag),
		Debug:           po.debugFlag,
		DebugPort:       po.EnvSpecificInfo.GetDebugPort(),
	}

	_, err = po.EnvSpecificInfo.ListURLs()
	if err != nil {
		return err
	}

	// Start or update the component
	err = devfileHandler.Push(pushParams)
	if err != nil {
		err = errors.Errorf("Failed to start component with name %q. Error: %v",
			componentName,
			err,
		)
	} else {
		log.Infof("\nPushing devfile component %q", componentName)
		log.Success("Changes successfully pushed to component")
	}

	return
}

// DevfileUnDeploy undeploys the devfile kubernetes components
func (do *DeleteOptions) DevfileUnDeploy() error {
	devObj, err := devfile.ParseAndValidateFromFile(do.GetDevfilePath())
	if err != nil {
		return err
	}

	componentName := do.EnvSpecificInfo.GetName()

	kc := kubernetes.KubernetesContext{
		Namespace: do.KClient.GetCurrentNamespace(),
	}

	devfileHandler, err := adapters.NewComponentAdapter(componentName, do.contextFlag, do.GetApplication(), devObj, kc)
	if err != nil {
		return err
	}

	return devfileHandler.UnDeploy()
}

// DevfileComponentDelete deletes the devfile component
func (do *DeleteOptions) DevfileComponentDelete() error {
	devObj, err := devfile.ParseAndValidateFromFile(do.GetDevfilePath())
	if err != nil {
		return err
	}

	componentName := do.EnvSpecificInfo.GetName()

	kc := kubernetes.KubernetesContext{
		Namespace: do.KClient.GetCurrentNamespace(),
	}

	labels := componentlabels.GetLabels(componentName, do.EnvSpecificInfo.GetApplication(), false)
	devfileHandler, err := adapters.NewComponentAdapter(componentName, do.contextFlag, do.GetApplication(), devObj, kc)
	if err != nil {
		return err
	}

	return devfileHandler.Delete(labels, do.showLogFlag, do.waitFlag)
}
