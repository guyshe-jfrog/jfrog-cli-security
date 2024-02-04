package audit

import (
	"errors"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-security/commands/audit/jas"
	"github.com/jfrog/jfrog-cli-security/commands/audit/jas/applicability"
	"github.com/jfrog/jfrog-cli-security/commands/audit/jas/iac"
	"github.com/jfrog/jfrog-cli-security/commands/audit/jas/sast"
	"github.com/jfrog/jfrog-cli-security/commands/audit/jas/secrets"
	"github.com/jfrog/jfrog-cli-security/utils"
	"github.com/jfrog/jfrog-client-go/utils/io"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func runJasScannersAndSetResults(scanResults *utils.Results, directDependencies []string,
	serverDetails *config.ServerDetails, workingDirs []string, progress io.ProgressMgr, thirdPartyApplicabilityScan bool) (err error) {
	if serverDetails == nil || len(serverDetails.Url) == 0 {
		print("To include 'Advanced Security' scan as part of the audit output, please run the 'jf c add\n")
		log.Warn("To include 'Advanced Security' scan as part of the audit output, please run the 'jf c add' command before running this command.")
		return
	}
	print("New jas scanner\n")
	scanner, err := jas.NewJasScanner(workingDirs, serverDetails)
	if err != nil {
		return
	}
	defer func() {
		cleanup := scanner.ScannerDirCleanupFunc
		err = errors.Join(err, cleanup())
	}()
	print("before Running applicability scanning\n")
	if progress != nil {
		progress.SetHeadlineMsg("Running applicability scanning")
	}
	scanResults.ExtendedScanResults.ApplicabilityScanResults, err = applicability.RunApplicabilityScan(scanResults.GetScaScansXrayResults(), directDependencies, scanResults.GetScaScannedTechnologies(), scanner, thirdPartyApplicabilityScan)
	if err != nil {
		return
	}
	print("After Running applicability scanning\n")
	// Don't execute other scanners when scanning third party dependencies.
	if thirdPartyApplicabilityScan {
		return
	}
	print("BeforeRunning secrets scanning\n")
	if progress != nil {
		progress.SetHeadlineMsg("Running secrets scanning")
	}
	scanResults.ExtendedScanResults.SecretsScanResults, err = secrets.RunSecretsScan(scanner, secrets.SecretsScannerType)
	if err != nil {
		return
	}
	if progress != nil {
		progress.SetHeadlineMsg("Running IaC scanning")
	}
	scanResults.ExtendedScanResults.IacScanResults, err = iac.RunIacScan(scanner)
	if err != nil {
		return
	}
	if progress != nil {
		progress.SetHeadlineMsg("Running SAST scanning")
	}
	scanResults.ExtendedScanResults.SastScanResults, err = sast.RunSastScan(scanner)
	return
}
