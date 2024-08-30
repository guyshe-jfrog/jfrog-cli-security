package cli

import (
	"errors"
	"fmt"
	buildInfoUtils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/common/cliutils"
	commandsCommon "github.com/jfrog/jfrog-cli-core/v2/common/commands"
	outputFormat "github.com/jfrog/jfrog-cli-core/v2/common/format"
	"github.com/jfrog/jfrog-cli-core/v2/common/progressbar"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	pluginsCommon "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/usage"
	enrichDocs "github.com/jfrog/jfrog-cli-security/cli/docs/enrich"
	"github.com/jfrog/jfrog-cli-security/commands/enrich"
	"github.com/jfrog/jfrog-cli-security/utils/xray"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/urfave/cli"
	"os"
	"strings"

	flags "github.com/jfrog/jfrog-cli-security/cli/docs"
	auditSpecificDocs "github.com/jfrog/jfrog-cli-security/cli/docs/auditspecific"
	auditDocs "github.com/jfrog/jfrog-cli-security/cli/docs/scan/audit"
	buildScanDocs "github.com/jfrog/jfrog-cli-security/cli/docs/scan/buildscan"
	curationDocs "github.com/jfrog/jfrog-cli-security/cli/docs/scan/curation"
	dockerScanDocs "github.com/jfrog/jfrog-cli-security/cli/docs/scan/dockerscan"
	scanDocs "github.com/jfrog/jfrog-cli-security/cli/docs/scan/scan"

	"github.com/jfrog/jfrog-cli-security/commands/audit"
	"github.com/jfrog/jfrog-cli-security/commands/curation"
	"github.com/jfrog/jfrog-cli-security/commands/scan"
	"github.com/jfrog/jfrog-cli-security/utils"
	"github.com/jfrog/jfrog-cli-security/utils/severityutils"
	"github.com/jfrog/jfrog-cli-security/utils/techutils"
	"github.com/jfrog/jfrog-cli-security/utils/xsc"
)

const dockerScanCmdHiddenName = "dockerscan"
const SkipCurationAfterFailureEnv = "JFROG_CLI_SKIP_CURATION_AFTER_FAILURE"

func getAuditAndScansCommands() []components.Command {
	return []components.Command{
		{
			Name:        "scan",
			Aliases:     []string{"s"},
			Flags:       flags.GetCommandFlags(flags.XrScan),
			Description: scanDocs.GetDescription(),
			Arguments:   scanDocs.GetArguments(),
			Category:    securityCategory,
			Action:      ScanCmd,
		},
		{
			Name:        "sbom-enrich",
			Aliases:     []string{"se"},
			Flags:       flags.GetCommandFlags(flags.Enrich),
			Description: enrichDocs.GetDescription(),
			Arguments:   enrichDocs.GetArguments(),
			Category:    securityCategory,
			Action:      EnrichCmd,
		},
		{
			Name:        "build-scan",
			Aliases:     []string{"bs"},
			Flags:       flags.GetCommandFlags(flags.BuildScan),
			Description: buildScanDocs.GetDescription(),
			Arguments:   buildScanDocs.GetArguments(),
			Category:    securityCategory,
			Action:      BuildScan,
		},
		{
			// this command is hidden and have no logic, it will be run to provide 'help' as a part of the buildtools CLI for 'docker' commands. ('jf docker scan')
			// CLI buildtools will run the command if requested: https://github.com/jfrog/jfrog-cli/blob/v2/buildtools/cli.go
			Name:        dockerScanCmdHiddenName,
			Flags:       flags.GetCommandFlags(flags.DockerScan),
			Description: dockerScanDocs.GetDescription(),
			Arguments:   dockerScanDocs.GetArguments(),
			UsageOptions: &components.UsageOptions{
				Usage:                     dockerScanDocs.Usage,
				ReplaceAutoGeneratedUsage: true,
			},
			Hidden: true,
		},
		{
			Name:        "audit",
			Aliases:     []string{"aud"},
			Flags:       flags.GetCommandFlags(flags.Audit),
			Description: auditDocs.GetDescription(),
			Category:    securityCategory,
			Action:      AuditCmd,
		},
		{
			Name:        "curation-audit",
			Aliases:     []string{"ca"},
			Flags:       flags.GetCommandFlags(flags.CurationAudit),
			Description: curationDocs.GetDescription(),
			Category:    securityCategory,
			Action:      CurationCmd,
		},

		// TODO: Deprecated commands (remove at next CLI major version)
		{
			Name:        "audit-mvn",
			Aliases:     []string{"am"},
			Flags:       flags.GetCommandFlags(flags.AuditMvn),
			Description: auditSpecificDocs.GetMvnDescription(),
			Action: func(c *components.Context) error {
				return AuditSpecificCmd(c, techutils.Maven)
			},
			Hidden: true,
		},
		{
			Name:        "audit-gradle",
			Aliases:     []string{"ag"},
			Flags:       flags.GetCommandFlags(flags.AuditGradle),
			Description: auditSpecificDocs.GetGradleDescription(),
			Action: func(c *components.Context) error {
				return AuditSpecificCmd(c, techutils.Gradle)
			},
			Hidden: true,
		},
		{
			Name:        "audit-npm",
			Aliases:     []string{"an"},
			Flags:       flags.GetCommandFlags(flags.AuditNpm),
			Description: auditSpecificDocs.GetNpmDescription(),
			Action: func(c *components.Context) error {
				return AuditSpecificCmd(c, techutils.Npm)
			},
			Hidden: true,
		},
		{
			Name:        "audit-go",
			Aliases:     []string{"ago"},
			Flags:       flags.GetCommandFlags(flags.AuditGo),
			Description: auditSpecificDocs.GetGoDescription(),
			Action: func(c *components.Context) error {
				return AuditSpecificCmd(c, techutils.Go)
			},
			Hidden: true,
		},
		{
			Name:        "audit-pip",
			Aliases:     []string{"ap"},
			Flags:       flags.GetCommandFlags(flags.AuditPip),
			Description: auditSpecificDocs.GetPipDescription(),
			Action: func(c *components.Context) error {
				return AuditSpecificCmd(c, techutils.Pip)
			},
			Hidden: true,
		},
		{
			Name:        "audit-pipenv",
			Aliases:     []string{"ape"},
			Flags:       flags.GetCommandFlags(flags.AuditPipenv),
			Description: auditSpecificDocs.GetPipenvDescription(),
			Action: func(c *components.Context) error {
				return AuditSpecificCmd(c, techutils.Pipenv)
			},
			Hidden: true,
		},
	}
}

func EnrichCmd(c *components.Context) error {
	if len(c.Arguments) == 0 {
		return pluginsCommon.PrintHelpAndReturnError("providing a file path argument is mandatory", c)
	}
	serverDetails, err := createServerDetailsWithConfigOffer(c)
	if err != nil {
		return err
	}
	if err = validateXrayContext(c, serverDetails); err != nil {
		return err
	}
	specFile := createDefaultScanSpec(c, addTrailingSlashToRepoPathIfNeeded(c))
	if err = spec.ValidateSpec(specFile.Files, false, false); err != nil {
		return err
	}
	threads, err := pluginsCommon.GetThreadsCount(c)
	if err != nil {
		return err
	}
	EnrichCmd := enrich.NewEnrichCommand().
		SetServerDetails(serverDetails).
		SetThreads(threads).
		SetSpec(specFile)
	return commandsCommon.Exec(EnrichCmd)
}

func ScanCmd(c *components.Context) error {
	if len(c.Arguments) == 0 && !c.IsFlagSet(flags.SpecFlag) {
		return pluginsCommon.PrintHelpAndReturnError("providing either a <source pattern> argument or the 'spec' option is mandatory", c)
	}
	serverDetails, err := createServerDetailsWithConfigOffer(c)
	if err != nil {
		return err
	}
	err = validateXrayContext(c, serverDetails)
	if err != nil {
		return err
	}
	var specFile *spec.SpecFiles
	if c.IsFlagSet(flags.SpecFlag) && len(c.GetStringFlagValue(flags.SpecFlag)) > 0 {
		specFile, err = pluginsCommon.GetFileSystemSpec(c)
		if err != nil {
			return err
		}
	} else {
		specFile = createDefaultScanSpec(c, addTrailingSlashToRepoPathIfNeeded(c))
	}
	err = spec.ValidateSpec(specFile.Files, false, false)
	if err != nil {
		return err
	}
	threads, err := pluginsCommon.GetThreadsCount(c)
	if err != nil {
		return err
	}
	format, err := outputFormat.GetOutputFormat(c.GetStringFlagValue(flags.OutputFormat))
	if err != nil {
		return err
	}
	pluginsCommon.FixWinPathsForFileSystemSourcedCmds(specFile, c)
	minSeverity, err := getMinimumSeverity(c)
	if err != nil {
		return err
	}
	scanCmd := scan.NewScanCommand().
		SetServerDetails(serverDetails).
		SetThreads(threads).
		SetSpec(specFile).
		SetOutputFormat(format).
		SetProject(getProject(c)).
		SetIncludeVulnerabilities(c.GetBoolFlagValue(flags.Vuln) || shouldIncludeVulnerabilities(c)).
		SetIncludeLicenses(c.GetBoolFlagValue(flags.Licenses)).
		SetFail(c.GetBoolFlagValue(flags.Fail)).
		SetPrintExtendedTable(c.GetBoolFlagValue(flags.ExtendedTable)).
		SetBypassArchiveLimits(c.GetBoolFlagValue(flags.BypassArchiveLimits)).
		SetFixableOnly(c.GetBoolFlagValue(flags.FixableOnly)).
		SetMinSeverityFilter(minSeverity)
	if c.IsFlagSet(flags.Watches) {
		scanCmd.SetWatches(splitByCommaAndTrim(c.GetStringFlagValue(flags.Watches)))
	}
	return commandsCommon.Exec(scanCmd)
}

func createServerDetailsWithConfigOffer(c *components.Context) (*coreConfig.ServerDetails, error) {
	return pluginsCommon.CreateServerDetailsWithConfigOffer(c, true, cliutils.Xr)
}

func validateXrayContext(c *components.Context, serverDetails *coreConfig.ServerDetails) error {
	if serverDetails.XrayUrl == "" {
		return errorutils.CheckErrorf("JFrog Xray URL must be provided in order run this command. Use the 'jf c add' command to set the Xray server details.")
	}
	contextFlag := 0
	if c.GetStringFlagValue(flags.Watches) != "" {
		contextFlag++
	}
	if isProjectProvided(c) {
		contextFlag++
	}
	if c.GetStringFlagValue(flags.RepoPath) != "" {
		contextFlag++
	}
	if contextFlag > 1 {
		return errorutils.CheckErrorf("only one of the following flags can be supplied: --watches, --project or --repo-path")
	}
	return nil
}

func getMinimumSeverity(c *components.Context) (severity severityutils.Severity, err error) {
	flagSeverity := c.GetStringFlagValue(flags.MinSeverity)
	if flagSeverity == "" {
		return
	}
	severity, err = severityutils.ParseSeverity(flagSeverity, false)
	if err != nil {
		return
	}
	return
}

func isProjectProvided(c *components.Context) bool {
	return getProject(c) != ""
}

func getProject(c *components.Context) string {
	if c.IsFlagSet(flags.Project) {
		return c.GetStringFlagValue(flags.Project)
	}
	return os.Getenv(coreutils.Project)
}

func addTrailingSlashToRepoPathIfNeeded(c *components.Context) string {
	repoPath := c.GetStringFlagValue(flags.RepoPath)
	if repoPath != "" && !strings.Contains(repoPath, "/") {
		// In case only repo name was provided (no path) we are adding a trailing slash.
		repoPath += "/"
	}
	return repoPath
}

func createDefaultScanSpec(c *components.Context, defaultTarget string) *spec.SpecFiles {
	return spec.NewBuilder().
		Pattern(c.Arguments[0]).
		Target(defaultTarget).
		Recursive(c.GetBoolFlagValue(flags.Recursive)).
		Exclusions(pluginsCommon.GetStringsArrFlagValue(c, flags.Exclusions)).
		Regexp(c.GetBoolFlagValue(flags.RegexpFlag)).
		Ant(c.GetBoolFlagValue(flags.AntFlag)).
		IncludeDirs(c.GetBoolFlagValue(flags.IncludeDirs)).
		BuildSpec()
}

func shouldIncludeVulnerabilities(c *components.Context) bool {
	// If no context was provided by the user, no Violations will be triggered by Xray, so include general vulnerabilities in the command output
	return c.GetStringFlagValue(flags.Watches) == "" && !isProjectProvided(c) && c.GetStringFlagValue(flags.RepoPath) == ""
}

func splitByCommaAndTrim(paramValue string) (res []string) {
	args := strings.Split(paramValue, ",")
	res = make([]string, len(args))
	for i, arg := range args {
		res[i] = strings.TrimSpace(arg)
	}
	return
}

// Scan published builds with Xray
func BuildScan(c *components.Context) error {
	if len(c.Arguments) > 2 {
		return pluginsCommon.WrongNumberOfArgumentsHandler(c)
	}
	buildConfiguration := pluginsCommon.CreateBuildConfiguration(c)
	if err := buildConfiguration.ValidateBuildParams(); err != nil {
		return err
	}
	serverDetails, err := createServerDetailsWithConfigOffer(c)
	if err != nil {
		return err
	}
	err = validateXrayContext(c, serverDetails)
	if err != nil {
		return err
	}
	format, err := outputFormat.GetOutputFormat(c.GetStringFlagValue(flags.OutputFormat))
	if err != nil {
		return err
	}
	buildScanCmd := scan.NewBuildScanCommand().
		SetServerDetails(serverDetails).
		SetFailBuild(c.GetBoolFlagValue(flags.Fail)).
		SetBuildConfiguration(buildConfiguration).
		SetOutputFormat(format).
		SetPrintExtendedTable(c.GetBoolFlagValue(flags.ExtendedTable)).
		SetRescan(c.GetBoolFlagValue(flags.Rescan))
	if format != outputFormat.Sarif {
		// Sarif shouldn't include the additional all-vulnerabilities info that received by adding the vuln flag
		buildScanCmd.SetIncludeVulnerabilities(c.GetBoolFlagValue(flags.Vuln))
	}
	return commandsCommon.Exec(buildScanCmd)
}

func AuditCmd(c *components.Context) error {
	auditCmd, err := CreateAuditCmd(c)
	if err != nil {
		return err
	}

	// Check if user used specific technologies flags
	allTechnologies := techutils.GetAllTechnologiesList()
	technologies := []string{}
	for _, tech := range allTechnologies {
		var techExists bool
		if tech == techutils.Maven {
			// On Maven we use '--mvn' flag
			techExists = c.GetBoolFlagValue(flags.Mvn)
		} else {
			techExists = c.GetBoolFlagValue(tech.String())
		}
		if techExists {
			technologies = append(technologies, tech.String())
		}
	}
	auditCmd.SetTechnologies(technologies)

	if c.GetBoolFlagValue(flags.WithoutCA) && !c.GetBoolFlagValue(flags.Sca) {
		// No CA flag provided but sca flag is not provided, error
		return pluginsCommon.PrintHelpAndReturnError(fmt.Sprintf("flag '--%s' cannot be used without '--%s'", flags.WithoutCA, flags.Sca), c)
	}

	allSubScans := utils.GetAllSupportedScans()
	subScans := []utils.SubScanType{}
	for _, subScan := range allSubScans {
		if shouldAddSubScan(subScan, c) {
			subScans = append(subScans, subScan)
		}
	}
	if len(subScans) > 0 {
		auditCmd.SetScansToPerform(subScans)
	}

	threads, err := pluginsCommon.GetThreadsCount(c)
	if err != nil {
		return err
	}
	auditCmd.SetThreads(threads)
	err = progressbar.ExecWithProgress(auditCmd)
	// Reporting error if Xsc service is enabled
	reportErrorIfExists(err, auditCmd)
	return err
}

func shouldAddSubScan(subScan utils.SubScanType, c *components.Context) bool {
	return c.GetBoolFlagValue(subScan.String()) ||
		(subScan == utils.ContextualAnalysisScan && c.GetBoolFlagValue(flags.Sca) && !c.GetBoolFlagValue(flags.WithoutCA))
}

func reportErrorIfExists(err error, auditCmd *audit.AuditCommand) {
	if err == nil || !usage.ShouldReportUsage() {
		return
	}
	var serverDetails *coreConfig.ServerDetails
	serverDetails, innerError := auditCmd.ServerDetails()
	if innerError != nil {
		log.Debug(fmt.Sprintf("failed to get server details for error report: %q", innerError))
		return
	}
	if reportError := xsc.ReportError(serverDetails, err, "cli"); reportError != nil {
		log.Debug("failed to report error log:" + reportError.Error())
	}
}

func CreateAuditCmd(c *components.Context) (*audit.AuditCommand, error) {
	auditCmd := audit.NewGenericAuditCommand()
	serverDetails, err := createServerDetailsWithConfigOffer(c)
	if err != nil {
		return nil, err
	}
	err = validateXrayContext(c, serverDetails)
	if err != nil {
		return nil, err
	}
	format, err := outputFormat.GetOutputFormat(c.GetStringFlagValue(flags.OutputFormat))
	if err != nil {
		return nil, err
	}
	minSeverity, err := getMinimumSeverity(c)
	if err != nil {
		return nil, err
	}
	auditCmd.SetAnalyticsMetricsService(xsc.NewAnalyticsMetricsService(serverDetails))

	auditCmd.SetTargetRepoPath(addTrailingSlashToRepoPathIfNeeded(c)).
		SetProject(getProject(c)).
		SetIncludeVulnerabilities(c.GetBoolFlagValue(flags.Vuln) || shouldIncludeVulnerabilities(c)).
		SetIncludeLicenses(c.GetBoolFlagValue(flags.Licenses)).
		SetFail(c.GetBoolFlagValue(flags.Fail)).
		SetPrintExtendedTable(c.GetBoolFlagValue(flags.ExtendedTable)).
		SetMinSeverityFilter(minSeverity).
		SetFixableOnly(c.GetBoolFlagValue(flags.FixableOnly)).
		SetThirdPartyApplicabilityScan(c.GetBoolFlagValue(flags.ThirdPartyContextualAnalysis))

	if c.GetStringFlagValue(flags.Watches) != "" {
		auditCmd.SetWatches(splitByCommaAndTrim(c.GetStringFlagValue(flags.Watches)))
	}

	if c.GetStringFlagValue(flags.WorkingDirs) != "" {
		auditCmd.SetWorkingDirs(splitByCommaAndTrim(c.GetStringFlagValue(flags.WorkingDirs)))
	}
	auditCmd.SetServerDetails(serverDetails).
		SetExcludeTestDependencies(c.GetBoolFlagValue(flags.ExcludeTestDeps)).
		SetOutputFormat(format).
		SetUseJas(true).
		SetUseWrapper(c.GetBoolFlagValue(flags.UseWrapper)).
		SetInsecureTls(c.GetBoolFlagValue(flags.InsecureTls)).
		SetNpmScope(c.GetStringFlagValue(flags.DepType)).
		SetPipRequirementsFile(c.GetStringFlagValue(flags.RequirementsFile)).
		SetExclusions(pluginsCommon.GetStringsArrFlagValue(c, flags.Exclusions))
	return auditCmd, err
}

func logNonGenericAuditCommandDeprecation(cmdName string) {
	if cliutils.ShouldLogWarning() {
		log.Warn(
			`You are using a deprecated syntax of the command.
	Instead of:
	$ ` + coreutils.GetCliExecutableName() + ` ` + cmdName + ` ...
	Use:
	$ ` + coreutils.GetCliExecutableName() + ` audit ...`)
	}
}

func AuditSpecificCmd(c *components.Context, technology techutils.Technology) error {
	logNonGenericAuditCommandDeprecation(c.CommandName)
	auditCmd, err := CreateAuditCmd(c)
	if err != nil {
		return err
	}
	technologies := []string{string(technology)}
	auditCmd.SetTechnologies(technologies)
	err = progressbar.ExecWithProgress(auditCmd)

	// Reporting error if Xsc service is enabled
	reportErrorIfExists(err, auditCmd)
	return err
}

func CurationCmd(c *components.Context) error {
	curationAuditCommand, err := getCurationCommand(c)
	if err != nil {
		return err
	}
	return progressbar.ExecWithProgress(curationAuditCommand)
}

var supportedCommandsForPostInstallationFailure = datastructures.MakeSetFromElements[string](
	"install", "build", "i", "add", "ci", "get", "mod",
)

func IsSupportedCommandForCurationInspect(cmd string) bool {
	return supportedCommandsForPostInstallationFailure.Exists(cmd)
}

func WrapCmdWithCurationPostFailureRun(c *cli.Context, cmd func(c *cli.Context) error, technology techutils.Technology, cmdName string) error {
	if err := cmd(c); err != nil {
		CurationInspectAfterFailure(c, cmdName, technology, err)
		return err
	}
	return nil
}

func CurationInspectAfterFailure(c *cli.Context, cmdName string, technology techutils.Technology, errFromCmd error) {
	if compContexts, errConvertCtx := components.ConvertContext(c); errConvertCtx == nil {
		if errPostCuration := CurationCmdPostInstallationFailure(compContexts, technology, cmdName, errFromCmd); errPostCuration != nil {
			log.Error(errPostCuration)
		}
	} else {
		log.Error(errConvertCtx)
	}
}

func CurationCmdPostInstallationFailure(c *components.Context, tech techutils.Technology, cmdName string, originError error) error {
	// check the command supported
	curationAuditCommand, err, runCuration := ShouldRunCurationAfterFailure(c, tech, cmdName, originError)
	if err != nil {
		return err
	}
	if !runCuration {
		return nil
	}
	log.Info("Running curation audit after failure")
	return progressbar.ExecWithProgress(curationAuditCommand)
}

func ShouldRunCurationAfterFailure(c *components.Context, tech techutils.Technology, cmdName string, originError error) (curationCmd *curation.CurationAuditCommand, err error, runCuration bool) {
	if !IsSupportedCommandForCurationInspect(cmdName) {
		return
	}
	if os.Getenv(coreutils.SummaryOutputDirPathEnv) == "" ||
		os.Getenv(SkipCurationAfterFailureEnv) == "true" {
		return
	}
	// check if the error is a forbidden error, if so, we don't want to run the curation audit automatically.
	// this check have two parts:
	// 1. check if the error is a forbidden error
	// 2. check if the error message contains the forbidden error message, in case the output included in the error message.
	forBiddenError := &buildInfoUtils.ForbiddenError{}
	if !errors.Is(originError, forBiddenError) && !strings.Contains(originError.Error(), forBiddenError.Error()) &&
		!buildInfoUtils.IsForbiddenOutput(buildInfoUtils.PackageManager(tech.String()), originError.Error()) {
		return
	}
	// If the command is not running in the context of GitHub actions, we don't want to run the curation audit automatically
	curationCmd, err = getCurationCommand(c)
	if err != nil {
		return
	}
	// check if user entitled for curation
	serverDetails, err := curationCmd.GetAuth(tech)
	if err != nil {
		return
	}
	xrayManager, err := xray.CreateXrayServiceManager(serverDetails)
	if err != nil {
		return
	}
	entitled, err := curation.IsEntitledForCuration(xrayManager)
	if err != nil {
		return
	}
	if !entitled {
		log.Info("Curation feature is not entitled, skipping curation audit")
		return
	}
	return curationCmd, nil, true
}

func getCurationCommand(c *components.Context) (*curation.CurationAuditCommand, error) {
	threads, err := pluginsCommon.GetThreadsCount(c)
	if err != nil {
		return nil, err
	}
	curationAuditCommand := curation.NewCurationAuditCommand().
		SetWorkingDirs(splitByCommaAndTrim(c.GetStringFlagValue(flags.WorkingDirs))).
		SetParallelRequests(threads)

	serverDetails, err := pluginsCommon.CreateServerDetailsWithConfigOffer(c, true, cliutils.Rt)
	if err != nil {
		return nil, err
	}
	format, err := curation.GetCurationOutputFormat(c.GetStringFlagValue(flags.OutputFormat))
	if err != nil {
		return nil, err
	}
	curationAuditCommand.SetServerDetails(serverDetails).
		SetIsCurationCmd(true).
		SetExcludeTestDependencies(c.GetBoolFlagValue(flags.ExcludeTestDeps)).
		SetOutputFormat(format).
		SetUseWrapper(c.GetBoolFlagValue(flags.UseWrapper)).
		SetInsecureTls(c.GetBoolFlagValue(flags.InsecureTls)).
		SetNpmScope(c.GetStringFlagValue(flags.DepType)).
		SetPipRequirementsFile(c.GetStringFlagValue(flags.RequirementsFile))
	return curationAuditCommand, nil
}

func DockerScanMockCommand() components.Command {
	// Mock how the CLI handles docker commands:
	// https://github.com/jfrog/jfrog-cli/blob/v2/buildtools/cli.go#L691
	return components.Command{
		Name:  "docker",
		Flags: flags.GetCommandFlags(flags.DockerScan),
		Action: func(c *components.Context) error {
			args := pluginsCommon.ExtractArguments(c)
			var cmd, image string
			// We may have prior flags before push/pull commands for the docker client.
			for _, arg := range args {
				if !strings.HasPrefix(arg, "-") {
					if cmd == "" {
						cmd = arg
					} else {
						image = arg
						break
					}
				}
			}
			if cmd != "scan" {
				return fmt.Errorf("unsupported command: %s", cmd)
			}
			return DockerScan(c, image)
		},
	}
}

func DockerScan(c *components.Context, image string) error {
	// Since this command is not registered normally, we need to handle printing 'help' here by ourselves.
	c.CommandName = dockerScanCmdHiddenName
	printHelp := pluginsCommon.GetPrintCurrentCmdHelp(c)
	if show, err := cliutils.ShowGenericCmdHelpIfNeeded(c.Arguments, printHelp); show || err != nil {
		return err
	}
	if image == "" {
		return printHelp()
	}
	// Run the command
	threads, err := pluginsCommon.GetThreadsCount(c)
	if err != nil {
		return err
	}
	serverDetails, err := createServerDetailsWithConfigOffer(c)
	if err != nil {
		return err
	}
	err = validateXrayContext(c, serverDetails)
	if err != nil {
		return err
	}
	containerScanCommand := scan.NewDockerScanCommand()
	format, err := outputFormat.GetOutputFormat(c.GetStringFlagValue(flags.OutputFormat))
	if err != nil {
		return err
	}
	minSeverity, err := getMinimumSeverity(c)
	if err != nil {
		return err
	}
	containerScanCommand.SetImageTag(image).
		SetTargetRepoPath(addTrailingSlashToRepoPathIfNeeded(c)).
		SetServerDetails(serverDetails).
		SetOutputFormat(format).
		SetProject(getProject(c)).
		SetIncludeVulnerabilities(c.GetBoolFlagValue(flags.Vuln) || shouldIncludeVulnerabilities(c)).
		SetIncludeLicenses(c.GetBoolFlagValue(flags.Licenses)).
		SetFail(c.GetBoolFlagValue(flags.Fail)).
		SetPrintExtendedTable(c.GetBoolFlagValue(flags.ExtendedTable)).
		SetBypassArchiveLimits(c.GetBoolFlagValue(flags.BypassArchiveLimits)).
		SetFixableOnly(c.GetBoolFlagValue(flags.FixableOnly)).
		SetMinSeverityFilter(minSeverity).
		SetThreads(threads).
		SetAnalyticsMetricsService(xsc.NewAnalyticsMetricsService(serverDetails))
	if c.GetStringFlagValue(flags.Watches) != "" {
		containerScanCommand.SetWatches(splitByCommaAndTrim(c.GetStringFlagValue(flags.Watches)))
	}
	return progressbar.ExecWithProgress(containerScanCommand)
}
