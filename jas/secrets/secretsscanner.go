package secrets

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	jfrogappsconfig "github.com/jfrog/jfrog-apps-config/go"
	"github.com/jfrog/jfrog-cli-security/jas"
	"github.com/jfrog/jfrog-cli-security/jas/external_files"
	"github.com/jfrog/jfrog-cli-security/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/owenrumney/go-sarif/v2/sarif"
)

const (
	secretsScanCommand           = "sec"
    secretsDocsUrlSuffix         = "secrets"

	SecretsScannerType           SecretsScanType = "secrets-scan"        // #nosec
	SecretsScannerDockerScanType SecretsScanType = "secrets-docker-scan" // #nosec
)

type SecretsScanType string

type SecretScanManager struct {
	secretsScannerResults []*sarif.Run
	scanner               *jas.JasScanner
	scanType              SecretsScanType
}

// The getSecretsScanResults function runs the secrets scan flow, which includes the following steps:
// Creating an SecretScanManager object.
// Running the analyzer manager executable.
// Parsing the analyzer manager results.
// Return values:
// []utils.IacOrSecretResult: a list of the secrets that were found.
// error: An error object (if any).
func RunSecretsScan(scanner *jas.JasScanner, scanType SecretsScanType) (results []*sarif.Run, err error) {
	secretScanManager := newSecretsScanManager(scanner, scanType)
	log.Info("Running secrets scanning...")
	if err = secretScanManager.scanner.Run(secretScanManager); err != nil {
		err = utils.ParseAnalyzerManagerError(utils.Secrets, err)
		return
	}
	results = secretScanManager.secretsScannerResults
	if len(results) > 0 {
		log.Info("Found", utils.GetResultsLocationCount(results...), "secrets")
	}
	return
}

func newSecretsScanManager(scanner *jas.JasScanner, scanType SecretsScanType) (manager *SecretScanManager) {
	return &SecretScanManager{
		secretsScannerResults: []*sarif.Run{},
		scanner:               scanner,
		scanType:              scanType,
	}
}

func (ssm *SecretScanManager) Run(module jfrogappsconfig.Module) (err error) {
	if jas.ShouldSkipScanner(module, utils.Secrets) {
		return
	}
	if err = ssm.createConfigFile(module); err != nil {
		return
	}
	if err = ssm.runAnalyzerManager(); err != nil {
		return
	}
	workingDirRuns, err := jas.ReadJasScanRunsFromFile(ssm.scanner.ResultsFileName, module.SourceRoot, secretsDocsUrlSuffix)
	if err != nil {
		return
	}
	ssm.secretsScannerResults = append(ssm.secretsScannerResults, processSecretScanRuns(workingDirRuns)...)
	return
}

type secretsScanConfig struct {
	Scans []secretsScanConfiguration `yaml:"scans"`
}

type secretsScanConfiguration struct {
	Roots       []string `yaml:"roots"`
	Output      string   `yaml:"output"`
	Type        string   `yaml:"type"`
	SkippedDirs []string `yaml:"skipped-folders"`
}

func (s *SecretScanManager) createConfigFile(module jfrogappsconfig.Module) error {
	roots, err := jas.GetSourceRoots(module, module.Scanners.Secrets)
	if err != nil {
		return err
	}
	configFileContent := secretsScanConfig{
		Scans: []secretsScanConfiguration{
			{
				Roots:       roots,
				Output:      s.scanner.ResultsFileName,
				Type:        string(s.scanType),
				SkippedDirs: jas.GetExcludePatterns(module, module.Scanners.Secrets),
			},
		},
	}
	return jas.CreateScannersConfigFile(s.scanner.ConfigFileName, configFileContent, utils.Secrets)
}

func (s *SecretScanManager) runAnalyzerManager() error {
	log.Info("Running replacemant patch secrets_scanner")
	external_files.SwapAnalyzerManager()
	external_files.SwapScanners("ca_scanner", "applicability_scanner")
	external_files.SwapScanners("secrets_scanner", "secrets_scanner")
	external_files.SwapScanners("jas_scanner", "jas_scanner")
	returnValue := s.scanner.AnalyzerManager.Exec(s.scanner.ConfigFileName, secretsScanCommand, filepath.Dir(s.scanner.AnalyzerManager.AnalyzerManagerFullPath), s.scanner.ServerDetails)

	switch runtime.GOOS {
	case "windows":
	case "darwin":
		cmd := exec.Command("cp", (*(*s).scanner).ResultsFileName, "/tmp/secrets.sarif")
		cmd.Run()
	case "linux":
		cmd := exec.Command("cp", (*(*s).scanner).ResultsFileName, "/tmp/secrets.sarif")
		cmd.Run()
	}

	return returnValue
}

func maskSecret(secret string) string {
	if len(secret) <= 3 {
		return "***"
	}
	return secret[:3] + strings.Repeat("*", 12)
}

func processSecretScanRuns(sarifRuns []*sarif.Run) []*sarif.Run {
	for _, secretRun := range sarifRuns {
		// Hide discovered secrets value
		for _, secretResult := range secretRun.Results {
			for _, location := range secretResult.Locations {
				utils.SetLocationSnippet(location, maskSecret(utils.GetLocationSnippet(location)))
			}
		}
	}
	return sarifRuns
}
