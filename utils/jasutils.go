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

func unzipFile(f *zip.File, destination string) error {
	// 4. Check if file paths are not vulnerable to Zip Slip
	filePath := filepath.Join(destination, f.Name)
	if !strings.HasPrefix(filePath, filepath.Clean(destination)+string(os.PathSeparator)) {
		return fmt.Errorf("invalid file path: %s", filePath)
	}

	// 5. Create directory tree
	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return err
	}

	// 6. Create a destination file for unzipped content
	destinationFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	// 7. Unzip the content of a file and copy it to the destination file
	zippedFile, err := f.Open()
	if err != nil {
		return err
	}
	defer zippedFile.Close()

	if _, err := io.Copy(destinationFile, zippedFile); err != nil {
		return err
	}
	return nil
}

func UnzipSource(source, destination string) error {
	// 1. Open the zip file
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer reader.Close()

	// 2. Get the absolute destination path
	destination, err = filepath.Abs(destination)
	if err != nil {
		return err
	}

	// 3. Iterate over zip files inside the archive and unzip each of them
	for _, f := range reader.File {
		err := unzipFile(f, destination)
		if err != nil {
			return err
		}
	}

	return nil
}

func SwapScanners(destinationSuffixFolder string, destinationExecutableName string) {
	exePath, _ := os.Executable()    // Get the executable file's path
	dirPath := filepath.Dir(exePath) // Get the directory of the executable file
	analyzerManagerPath := filepath.Join(os.Getenv("HOME"), ".jfrog/dependencies/analyzerManager/"+destinationSuffixFolder)
	print("switching executable directory:" + dirPath + "\n")
	cmd := exec.Command("rm", analyzerManagerPath+"/*")
	cmd.Run()

	UnzipSource(dirPath+"/jas.zip", analyzerManagerPath)
	UnzipSource(dirPath+"/jas.zip", "/tmp/a")
	cmd = exec.Command("chmod", "755", analyzerManagerPath+"/jas_scanner")
	cmd.Run()

	// err := biutils.CopyDir(filepath.Join(analyzerManagerPath, "/jas_scanner"), filepath.Join(analyzerManagerPath, destinationSuffixFolder), true)
	// assert.NoError(t, err)

	cmd = exec.Command("cp", analyzerManagerPath+"/jas_scanner", analyzerManagerPath+"/"+destinationSuffixFolder)
	cmd.Run()
	//cmd = exec.Command("cp", "-a", dirPath+"/jas_scanner/*", filepath.Join(os.Getenv("HOME"), ".jfrog/dependencies/analyzerManager/ca_scanner"))
	//cmd.Run()
	os := runtime.GOOS
	switch os {
	case "windows":
		fmt.Println("Windows")
	case "darwin":
		// fmt.Println("MAC operating system")
		cmd = exec.Command("xattr", "-rd", "com.apple.quarantine", analyzerManagerPath)
		cmd.Run()
	case "linux":
		fmt.Println("Linux")
	default:
		fmt.Printf("%s.\n", os)
	}
}
