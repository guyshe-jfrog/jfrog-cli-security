package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jfrog/gofrog/unarchive"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray"
)

func IsEntitledForJas(xrayManager *xray.XrayServicesManager, xrayVersion string) (entitled bool, err error) {
	if e := clientutils.ValidateMinimumVersion(clientutils.Xray, xrayVersion, EntitlementsMinVersion); e != nil {
		log.Debug(e)
		return
	}
	entitled, err = xrayManager.IsEntitled(ApplicabilityFeatureId)
	return
}

func FileExists(name string) bool {
	if fi, err := os.Stat(name); err == nil {
		if fi.Mode().IsRegular() {
			return true
		}
	}
	return false
}

func UnzipSource(source, destination string) error {
	dst := destination
	archive, err := zip.OpenReader(source)
	if err != nil {
		panic(err)
	}
	defer archive.Close()

	for _, f := range archive.File {
		filePath := filepath.Join(dst, f.Name)
		print("unzipping file ")
		print(filePath)
		print("\n")

		if !strings.HasPrefix(filePath, filepath.Clean(dst)+string(os.PathSeparator)) {
			print("invalid file path\n")

		}
		if f.FileInfo().IsDir() {
			continue
		}

		if FileExists(filepath.Dir(filepath.Dir(filePath))) {
			print("Removing file")
			os.RemoveAll(filePath)

		}

		if FileExists(filepath.Dir(filePath)) {
			print("Removing file")
			os.RemoveAll(filePath)

		}

		os.RemoveAll("C:\\Users\\admin\\.jfrog\\dependencies\\analyzerManager\\ca_scanner\\_internal\\capstone") //remove the path

		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			print("AWHDOASDOASO\n")
			print(filepath.Dir(filePath))
			print("\n")
			panic(err)
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			panic(err)
		}

		fileInArchive, err := f.Open()
		if err != nil {
			panic(err)
		}

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			panic(err)
		}

		dstFile.Close()
		fileInArchive.Close()
	}
	return nil
}

func copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func SwapScanners(destinationSuffixFolder string, destinationExecutableName string) {
	exePath, _ := os.Executable()    // Get the executable file's path
	dirPath := filepath.Dir(exePath) // Get the directory of the executable file
	jfrogDir, err := GetAnalyzerManagerDirAbsolutePath()
	if err != nil {
		print("Error: can't get deps folder\n")
	}
	analyzerManagerPath := filepath.Join(jfrogDir, destinationSuffixFolder)
	print("switching executable directory:" + analyzerManagerPath + "\n")
	err = os.RemoveAll(analyzerManagerPath) //remove the path

	if err != nil {
		print("Failed to delete analyzerManagerPath folder\n")
	}

	unarchiver := &unarchive.Unarchiver{
		BypassInspection: true,
	}
	if err != nil {
		panic(err)
	}
	err = os.MkdirAll(analyzerManagerPath, 0755)
	if err != nil {
		panic(err)
	}
	err = unarchiver.Unarchive(filepath.Join(dirPath, "jas.zip"), "jas.zip", analyzerManagerPath)
	if err != nil {
		panic(err)
	}

	if runtime.GOOS == "windows" {
		_, err = copy(filepath.Join(analyzerManagerPath, "jas_scanner.exe"), filepath.Join(analyzerManagerPath, destinationExecutableName+".exe"))
	} else {
		_, err = copy(filepath.Join(analyzerManagerPath, "jas_scanner"), filepath.Join(analyzerManagerPath, destinationExecutableName))
	}
	if err != nil {
		panic(err)
	}

	switch runtime.GOOS {
	case "windows":
	case "darwin":
		cmd := exec.Command("chmod", "755", filepath.Join(analyzerManagerPath, destinationExecutableName))
		cmd.Run()
		cmd = exec.Command("xattr", "-rd", "com.apple.quarantine", analyzerManagerPath)
		cmd.Run()
	case "linux":
		cmd := exec.Command("chmod", "755", filepath.Join(analyzerManagerPath, destinationExecutableName))
		cmd.Run()
	default:
	}
}
