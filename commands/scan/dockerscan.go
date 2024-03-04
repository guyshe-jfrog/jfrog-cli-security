package scan

import (
	"bytes"
	"fmt"
	xscservices "github.com/jfrog/jfrog-client-go/xsc/services"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-security/utils"
	"github.com/jfrog/jfrog-cli-security/utils/xray"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	indexerEnvPrefix         = "JFROG_INDEXER_"
	DockerScanMinXrayVersion = "3.40.0"
)

type DockerScanCommand struct {
	ScanCommand
	imageTag       string
	targetRepoPath string
}

func NewDockerScanCommand() *DockerScanCommand {
	return &DockerScanCommand{ScanCommand: *NewScanCommand()}
}

func (dsc *DockerScanCommand) SetImageTag(imageTag string) *DockerScanCommand {
	dsc.imageTag = imageTag
	return dsc
}

func (dsc *DockerScanCommand) SetTargetRepoPath(repoPath string) *DockerScanCommand {
	dsc.targetRepoPath = repoPath
	return dsc
}

func (dsc *DockerScanCommand) SetIsTar(isTar bool) *DockerScanCommand {
	dsc.isTar = isTar
	return dsc
}

func (dsc *DockerScanCommand) Run() (err error) {
	// Validate Xray minimum version
	_, xrayVersion, err := xray.CreateXrayServiceManagerAndGetVersion(dsc.ScanCommand.serverDetails)
	if err != nil {
		return err
	}
	err = clientutils.ValidateMinimumVersion(clientutils.Xray, xrayVersion, DockerScanMinXrayVersion)
	if err != nil {
		return err
	}

	tempDirPath, err := fileutils.CreateTempDir()
	if err != nil {
		return err
	}
	defer func() {
		e := fileutils.RemoveTempDir(tempDirPath)
		if err == nil {
			err = e
		}
	}()

	// Run the 'docker save' command, to create tar file from the docker image, and pass it to the indexer-app
	imageTarPath := filepath.Join(tempDirPath, "image.tar")
	if dsc.isTar {
		imageTarPath = dsc.imageTag
		exists, err := fileutils.IsFileExists(dsc.imageTag, false)
		if err != nil || !exists {
			return fmt.Errorf("failed to determine if tar file provided exists")
		}
	}

	if !dsc.isTar {
		if dsc.progress != nil {
			dsc.progress.SetHeadlineMsg("Creating image archive 📦")
		}
		log.Info("Creating image archive...")
		dockerSaveCmd := exec.Command("docker", "save", dsc.imageTag, "-o", imageTarPath)
		var stderr bytes.Buffer
		dockerSaveCmd.Stderr = &stderr
		err = dockerSaveCmd.Run()
		if err != nil {
			return fmt.Errorf("failed running command: '%s' with error: %s - %s", strings.Join(dockerSaveCmd.Args, " "), err.Error(), stderr.String())
		}
	}
	// Perform scan on image.tar
	dsc.analyticsMetricsService.AddGeneralEvent(dsc.analyticsMetricsService.CreateGeneralEvent(xscservices.CliProduct, xscservices.CliEventType))
	dsc.SetSpec(spec.NewBuilder().
		Pattern(imageTarPath).
		Target(dsc.targetRepoPath).
		BuildSpec()).SetThreads(1)
	dsc.ScanCommand.SetRunJasScans(true)
	err = dsc.setCredentialEnvsForIndexerApp()
	if err != nil {
		return errorutils.CheckError(err)
	}
	defer func() {
		e := dsc.unsetCredentialEnvsForIndexerApp()
		if err == nil {
			err = errorutils.CheckError(e)
		}
	}()
	return dsc.ScanCommand.RunAndRecordResults(utils.DockerImage, func(scanResults *utils.Results) (err error) {
		if scanResults == nil {
			return
		}
		if scanResults.ScaResults != nil {
			for _, result := range scanResults.ScaResults {
				result.Name = dsc.imageTag
			}
		}
		dsc.analyticsMetricsService.UpdateGeneralEvent(dsc.analyticsMetricsService.CreateXscAnalyticsGeneralEventFinalizeFromAuditResults(scanResults))
		if err = utils.RecordSarifOutput(scanResults, utils.GetAllSupportedScans()); err != nil {
			return
		}
		return utils.RecordSecurityCommandSummary(utils.NewDockerScanSummary(
			scanResults,
			dsc.ScanCommand.serverDetails,
			dsc.ScanCommand.includeVulnerabilities,
			dsc.ScanCommand.hasViolationContext(),
			dsc.imageTag,
		))
	})
}

// When indexing RPM files inside the docker container, the indexer-app needs to connect to the Xray Server.
// This is because RPM indexing is performed on the server side. This method therefore sets the Xray credentials as env vars to be read and used by the indexer-app.
func (dsc *DockerScanCommand) setCredentialEnvsForIndexerApp() error {
	err := os.Setenv(indexerEnvPrefix+"XRAY_URL", dsc.serverDetails.XrayUrl)
	if err != nil {
		return err
	}
	if dsc.serverDetails.AccessToken != "" {
		err = os.Setenv(indexerEnvPrefix+"XRAY_ACCESS_TOKEN", dsc.serverDetails.AccessToken)
		if err != nil {
			return err
		}
	} else {
		err = os.Setenv(indexerEnvPrefix+"XRAY_USER", dsc.serverDetails.User)
		if err != nil {
			return err
		}
		err = os.Setenv(indexerEnvPrefix+"XRAY_PASSWORD", dsc.serverDetails.Password)
		if err != nil {
			return err
		}
	}
	return nil
}

func (dsc *DockerScanCommand) unsetCredentialEnvsForIndexerApp() error {
	err := os.Unsetenv(indexerEnvPrefix + "XRAY_URL")
	if err != nil {
		return err
	}
	err = os.Unsetenv(indexerEnvPrefix + "XRAY_ACCESS_TOKEN")
	if err != nil {
		return err
	}
	err = os.Unsetenv(indexerEnvPrefix + "XRAY_USER")
	if err != nil {
		return err
	}
	err = os.Unsetenv(indexerEnvPrefix + "XRAY_PASSWORD")
	if err != nil {
		return err
	}

	return nil
}

func (dsc *DockerScanCommand) CommandName() string {
	return "xr_docker_scan"
}
