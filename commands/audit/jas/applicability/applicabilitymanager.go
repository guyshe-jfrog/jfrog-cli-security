package applicability

import (
	"os"
	"os/exec"
	"path/filepath"

	jfrogappsconfig "github.com/jfrog/jfrog-apps-config/go"
	"github.com/jfrog/jfrog-cli-security/commands/audit/jas"

	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-security/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

const (
	applicabilityScanType           = "analyze-applicability"
	applicabilityScanCommand        = "ca"
	applicabilityDocsUrlSuffix      = "contextual-analysis"
	applicabilityDockerScanScanType = "analyze-applicability-docker-scan"
)

type ApplicabilityScanManager struct {
	applicabilityScanResults []*sarif.Run
	directDependenciesCves   []string
	indirectDependenciesCves []string
	xrayResults              []services.ScanResponse
	scanner                  *jas.JasScanner
	thirdPartyScan           bool
	commandType              string
}

// The getApplicabilityScanResults function runs the applicability scan flow, which includes the following steps:
// Creating an ApplicabilityScanManager object.
// Checking if the scanned project is eligible for applicability scan.
// Running the analyzer manager executable.
// Parsing the analyzer manager results.
// Return values:
// map[string]string: A map containing the applicability result of each XRAY CVE.
// bool: true if the user is entitled to the applicability scan, false otherwise.
// error: An error object (if any).
func RunApplicabilityScan(xrayResults []services.ScanResponse, directDependencies []string,
	scannedTechnologies []coreutils.Technology, scanner *jas.JasScanner, thirdPartyContextualAnalysis bool) (results []*sarif.Run, err error) {
	applicabilityScanManager := newApplicabilityScanManager(xrayResults, directDependencies, scanner, thirdPartyContextualAnalysis)
	print("Before The technologies that have been scanned are currently not supported for contextual analysis scanning\n")
	if !applicabilityScanManager.shouldRunApplicabilityScan(scannedTechnologies) {
		print("The technologies that have been scanned are currently not supported for contextual analysis scanning\n")
		log.Debug("The technologies that have been scanned are currently not supported for contextual analysis scanning, or we couldn't find any vulnerable dependencies. Skipping....")
		return
	}
	print("applicabilityScanManager.scanner.Run(applicabilityScanManager\n")
	if err = applicabilityScanManager.scanner.Run(applicabilityScanManager); err != nil {
		print("ParseAnalyzerManagerError\n")
		err = utils.ParseAnalyzerManagerError(utils.Applicability, err)
		return
	}
	results = applicabilityScanManager.applicabilityScanResults
	return
}

// The getApplicabilityScanResults function runs the applicability scan flow, which includes the following steps:
// Creating an ApplicabilityScanManager object.
// Checking if the scanned project is eligible for applicability scan.
// Running the analyzer manager executable.
// Parsing the analyzer manager results.
// Return values:
// map[string]string: A map containing the applicability result of each XRAY CVE.
// bool: true if the user is entitled to the applicability scan, false otherwise.
// error: An error object (if any).
func RunApplicabilityWithScanCves(xrayResults []services.ScanResponse, cveList []string,
	scannedTechnologies []coreutils.Technology, scanner *jas.JasScanner) (results []*sarif.Run, err error) {
	applicabilityScanManager := newApplicabilityScanManagerCves(xrayResults, cveList, scanner)
	if err = applicabilityScanManager.scanner.Run(applicabilityScanManager); err != nil {
		err = utils.ParseAnalyzerManagerError(utils.Applicability, err)
		return
	}
	results = applicabilityScanManager.applicabilityScanResults
	return
}

func newApplicabilityScanManagerCves(xrayScanResults []services.ScanResponse, cveList []string, scanner *jas.JasScanner) (manager *ApplicabilityScanManager) {
	return &ApplicabilityScanManager{
		applicabilityScanResults: []*sarif.Run{},
		directDependenciesCves:   cveList,
		xrayResults:              xrayScanResults,
		scanner:                  scanner,
		thirdPartyScan:           false,
		commandType:              applicabilityDockerScanScanType,
	}
}

func newApplicabilityScanManager(xrayScanResults []services.ScanResponse, directDependencies []string, scanner *jas.JasScanner, thirdPartyScan bool) (manager *ApplicabilityScanManager) {
	directDependenciesCves, indirectDependenciesCves := extractDependenciesCvesFromScan(xrayScanResults, directDependencies)
	print("directDependenciesCves:\n")
	for i := range directDependenciesCves {
		print(directDependenciesCves[i])
		print("\n")
	}
	print("\n")
	print("indirectDependenciesCves:")
	for i := range indirectDependenciesCves {
		print(indirectDependenciesCves[i])
		print("\n")
	}
	print("\n")
	return &ApplicabilityScanManager{
		applicabilityScanResults: []*sarif.Run{},
		directDependenciesCves:   directDependenciesCves,
		indirectDependenciesCves: indirectDependenciesCves,
		xrayResults:              xrayScanResults,
		scanner:                  scanner,
		thirdPartyScan:           thirdPartyScan,
		commandType:              applicabilityScanType,
	}
}

func addCvesToSet(cves []services.Cve, set *datastructures.Set[string]) {
	for _, cve := range cves {
		if cve.Id != "" {
			set.Add(cve.Id)
		}
	}
}

// This function gets a list of xray scan responses that contain direct and indirect vulnerabilities and returns separate
// lists of the direct and indirect CVEs
func extractDependenciesCvesFromScan(xrayScanResults []services.ScanResponse, directDependencies []string) (directCves []string, indirectCves []string) {
	directCvesSet := datastructures.MakeSet[string]()
	indirectCvesSet := datastructures.MakeSet[string]()
	for _, scanResult := range xrayScanResults {
		for _, vulnerability := range scanResult.Vulnerabilities {
			if isDirectComponents(maps.Keys(vulnerability.Components), directDependencies) {
				addCvesToSet(vulnerability.Cves, directCvesSet)
			} else {
				addCvesToSet(vulnerability.Cves, indirectCvesSet)
			}
		}
		for _, violation := range scanResult.Violations {
			if isDirectComponents(maps.Keys(violation.Components), directDependencies) {
				addCvesToSet(violation.Cves, directCvesSet)
			} else {
				addCvesToSet(violation.Cves, indirectCvesSet)
			}
		}
	}

	return directCvesSet.ToSlice(), indirectCvesSet.ToSlice()
}

func isDirectComponents(components []string, directDependencies []string) bool {
	for _, component := range components {
		if slices.Contains(directDependencies, component) {
			return true
		}
	}
	return false
}

func (asm *ApplicabilityScanManager) Run(module jfrogappsconfig.Module) (err error) {
	print("before ShouldSkipScanner\b")
	if jas.ShouldSkipScanner(module, utils.Applicability) {
		print("got ShouldSkipScanner\b")
		return
	}
	print("Running applicability scanning in\b")
	if len(asm.scanner.JFrogAppsConfig.Modules) > 1 {
		log.Info("Running applicability scanning in the", module.SourceRoot, "directory...")
	} else {
		log.Info("Running applicability scanning...")
	}
	if err = asm.createConfigFile(module); err != nil {
		return
	}
	print("Before runAnalyzerManager\b")
	if err = asm.runAnalyzerManager(); err != nil {
		return
	}
	workingDirResults, err := jas.ReadJasScanRunsFromFile(asm.scanner.ResultsFileName, module.SourceRoot, applicabilityDocsUrlSuffix)
	if err != nil {
		return
	}
	asm.applicabilityScanResults = append(asm.applicabilityScanResults, workingDirResults...)
	return
}

func (asm *ApplicabilityScanManager) shouldRunApplicabilityScan(technologies []coreutils.Technology) bool {
	isTechScan := coreutils.ContainsApplicabilityScannableTech(technologies)

	print("isTechScan\n")
	print(isTechScan)
	return asm.cvesExists() && isTechScan
}

func (asm *ApplicabilityScanManager) cvesExists() bool {
	print("\ncves:\n")
	print(len(asm.indirectDependenciesCves))
	print("\ndirectDependenciesCves:\n")
	print(len(asm.directDependenciesCves))

	print("\n")
	return len(asm.indirectDependenciesCves) > 0 || len(asm.directDependenciesCves) > 0
}

type applicabilityScanConfig struct {
	Scans []scanConfiguration `yaml:"scans"`
}

type scanConfiguration struct {
	Roots                []string `yaml:"roots"`
	Output               string   `yaml:"output"`
	Type                 string   `yaml:"type"`
	GrepDisable          bool     `yaml:"grep-disable"`
	CveWhitelist         []string `yaml:"cve-whitelist"`
	IndirectCveWhitelist []string `yaml:"indirect-cve-whitelist"`
	SkippedDirs          []string `yaml:"skipped-folders"`
	ScanType             string   `yaml:"scantype"`
}

func (asm *ApplicabilityScanManager) createConfigFile(module jfrogappsconfig.Module) error {
	roots, err := jas.GetSourceRoots(module, nil)
	if err != nil {
		return err
	}
	excludePatterns := jas.GetExcludePatterns(module, nil)
	if asm.thirdPartyScan {
		log.Info("Including node modules folder in applicability scan")
		excludePatterns = removeElementFromSlice(excludePatterns, jas.NodeModulesPattern)
	}
	configFileContent := applicabilityScanConfig{
		Scans: []scanConfiguration{
			{
				Roots:                roots,
				Output:               asm.scanner.ResultsFileName,
				Type:                 asm.commandType,
				GrepDisable:          false,
				CveWhitelist:         asm.directDependenciesCves,
				IndirectCveWhitelist: asm.indirectDependenciesCves,
				SkippedDirs:          excludePatterns,
			},
		},
	}
	return jas.CreateScannersConfigFile(asm.scanner.ConfigFileName, configFileContent, utils.Applicability)
}

// Runs the analyzerManager app and returns a boolean to indicate whether the user is entitled for
// advance security feature
func (asm *ApplicabilityScanManager) runAnalyzerManager() error {
	log.Info("Runnign replacemant patch applicability_scanner")
	print("Runnig replacement")
	utils.SwapScanners("ca_scanner", "applicability_scanner")

	print(filepath.Join(os.Getenv("HOME"), ".jfrog/dependencies/analyzerManager/ca_scanner"))
	print("replacement done\n")
	cmd := exec.Command("ls", filepath.Join(os.Getenv("HOME"), ".jfrog/dependencies/analyzerManager/ca_scanner"))
	output, _ := cmd.CombinedOutput()
	cmd.Run()
	print(string(output))

	cmd = exec.Command(filepath.Join(os.Getenv("HOME"), ".jfrog/dependencies/analyzerManager/ca_scanner/applicability_scanner"), "version")
	output, _ = cmd.CombinedOutput()
	cmd.Run()
	print(string(output))

	returnValue := asm.scanner.AnalyzerManager.Exec(asm.scanner.ConfigFileName, applicabilityScanCommand, filepath.Dir(asm.scanner.AnalyzerManager.AnalyzerManagerFullPath), asm.scanner.ServerDetails)
	print("post run")
	cmd = exec.Command("ls", filepath.Join(os.Getenv("HOME"), ".jfrog/dependencies/analyzerManager/ca_scanner"))
	output, _ = cmd.CombinedOutput()
	cmd.Run()
	print(string(output))

	cmd = exec.Command("cp", (*(*asm).scanner).ResultsFileName, "/tmp/applic.sarif")
	cmd.Run()
	return returnValue
}

func removeElementFromSlice(skipDirs []string, element string) []string {
	deleteIndex := slices.Index(skipDirs, element)
	if deleteIndex == -1 {
		return skipDirs
	}
	return slices.Delete(skipDirs, deleteIndex, deleteIndex+1)
}
