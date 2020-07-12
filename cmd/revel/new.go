// Copyright (c) 2012-2016 The Revel Framework Authors, All rights reserved.
// Revel Framework source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"go/build"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/revel/cmd/model"
	"github.com/revel/cmd/utils"
	"net/url"
)

var cmdNew = &Command{
	UsageLine: "new -i [path] -s [skeleton]",
	Short:     "create a skeleton Revel application",
	Long: `
New creates a few files to get a new Revel application running quickly.

It puts all of the files in the given import path, taking the final element in
the path to be the app name.

Skeleton is an optional argument, provided as an import path

For example:

    revel new -a import/path/helloworld

    revel new -a import/path/helloworld -s import/path/skeleton

`,
}

func init() {
	cmdNew.RunWith = newApp
	cmdNew.UpdateConfig = updateNewConfig
}

// Called when unable to parse the command line automatically and assumes an old launch
func updateNewConfig(c *model.CommandConfig, args []string) bool {
	c.Index = model.NEW
	if len(c.New.Package) > 0 {
		c.New.NotVendored = false
	}
	c.Vendored = !c.New.NotVendored

	if len(args) == 0 {
		if len(c.New.ImportPath) == 0 {
			fmt.Fprintf(os.Stderr, cmdNew.Long)
			return false
		}
		return true
	}
	c.New.ImportPath = args[0]
	if len(args) > 1 {
		c.New.SkeletonPath = args[1]
	}

	return true

}

// Call to create a new application
func newApp(c *model.CommandConfig) (err error) {
	// Check for an existing folder so we don't clobber it
	_, err = build.Import(c.ImportPath, "", build.FindOnly)
	if err == nil || !utils.Empty(c.AppPath) {
		return utils.NewBuildError("Abort: Import path already exists.", "path", c.ImportPath, "apppath", c.AppPath)
	}

	// checking and setting skeleton
	if err = setSkeletonPath(c); err != nil {
		return
	}

	// Create application path
	if err := os.MkdirAll(c.AppPath, os.ModePerm); err != nil {
		return utils.NewBuildError("Abort: Unable to create app path.", "path", c.AppPath)
	}

	// checking and setting application
	if err = setApplicationPath(c); err != nil {
		return err
	}

	// This kicked off the download of the revel app, not needed for vendor
	if !c.Vendored {
		// At this point the versions can be set
		c.SetVersions()
	}

	// copy files to new app directory
	if err = copyNewAppFiles(c); err != nil {
		return
	}

	// Run the vendor tool if needed
	if c.Vendored {
		if err = createModVendor(c); err != nil {
			return
		}
	}

	// goodbye world
	fmt.Fprintln(os.Stdout, "Your application has been created in:\n  ", c.AppPath)
	// Check to see if it should be run right off
	if c.New.Run {
		// Need to prep the run command
		c.Run.ImportPath = c.ImportPath
		updateRunConfig(c,nil)
		c.UpdateImportPath()
		runApp(c)
	} else {
		fmt.Fprintln(os.Stdout, "\nYou can run it with:\n   revel run -a ", c.ImportPath)
	}
	return
}

func createModVendor(c *model.CommandConfig) (err error) {

	utils.Logger.Info("Creating a new mod app")
	goModCmd := exec.Command("go", "mod", "init", filepath.Join(c.New.Package, c.AppName))

	utils.CmdInit(goModCmd, !c.Vendored, c.AppPath)

	utils.Logger.Info("Exec:", "args", goModCmd.Args, "env", goModCmd.Env, "workingdir", goModCmd.Dir)
	getOutput, err := goModCmd.CombinedOutput()
	if c.New.Callback != nil {
		err = c.New.Callback()
	}
	if err != nil {
		return utils.NewBuildIfError(err, string(getOutput))
	}
	return
}

func createDepVendor(c *model.CommandConfig) (err error) {

	utils.Logger.Info("Creating a new vendor app")

	vendorPath := filepath.Join(c.AppPath, "vendor")
	if !utils.DirExists(vendorPath) {

		if err := os.MkdirAll(vendorPath, os.ModePerm); err != nil {
			return utils.NewBuildError("Failed to create " + vendorPath, "error", err)
		}
	}

	// In order for dep to run there needs to be a source file in the folder
	tempPath := filepath.Join(c.AppPath, "tmp")
	utils.Logger.Info("Checking for temp folder for source code", "path", tempPath)
	if !utils.DirExists(tempPath) {
		if err := os.MkdirAll(tempPath, os.ModePerm); err != nil {
			return utils.NewBuildIfError(err, "Failed to create " + vendorPath)
		}

		if err = utils.GenerateTemplate(filepath.Join(tempPath, "main.go"), NEW_MAIN_FILE, nil); err != nil {
			return utils.NewBuildIfError(err, "Failed to create main file " + vendorPath)
		}
	}

	// Create a package template file if it does not exist
	packageFile := filepath.Join(c.AppPath, "Gopkg.toml")
	utils.Logger.Info("Checking for Gopkg.toml", "path", packageFile)
	if !utils.Exists(packageFile) {
		utils.Logger.Info("Generating Gopkg.toml", "path", packageFile)
		if err := utils.GenerateTemplate(packageFile, VENDOR_GOPKG, nil); err != nil {
			return utils.NewBuildIfError(err, "Failed to generate template")
		}
	} else {
		utils.Logger.Info("Package file exists in skeleto, skipping adding")
	}

	getCmd := exec.Command("dep", "ensure", "-v")
	utils.CmdInit(getCmd, !c.Vendored, c.AppPath)

	utils.Logger.Info("Exec:", "args", getCmd.Args, "env", getCmd.Env, "workingdir", getCmd.Dir)
	getOutput, err := getCmd.CombinedOutput()
	if err != nil {
		return utils.NewBuildIfError(err, string(getOutput))
	}
	return
}

// Used to generate a new secret key
const alphaNumeric = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// Generate a secret key
func generateSecret() string {
	chars := make([]byte, 64)
	for i := 0; i < 64; i++ {
		chars[i] = alphaNumeric[rand.Intn(len(alphaNumeric))]
	}
	return string(chars)
}

// Sets the applicaiton path
func setApplicationPath(c *model.CommandConfig) (err error) {

	// revel/revel#1014 validate relative path, we cannot use built-in functions
	// since Go import path is valid relative path too.
	// so check basic part of the path, which is "."

	// If we are running a vendored version of Revel we do not need to check for it.
	if !c.Vendored {
		if filepath.IsAbs(c.ImportPath) || strings.HasPrefix(c.ImportPath, ".") {
			utils.Logger.Fatalf("Abort: '%s' looks like a directory.  Please provide a Go import path instead.",
				c.ImportPath)
		}
		_, err = build.Import(model.RevelImportPath, "", build.FindOnly)
		if err != nil {
			//// Go get the revel project
			err = c.PackageResolver(model.RevelImportPath)
			if err != nil {
				return utils.NewBuildIfError(err, "Failed to fetch revel " + model.RevelImportPath)
			}
		}
	}

	c.AppName = filepath.Base(c.AppPath)

	return nil
}

// Set the skeleton path
func setSkeletonPath(c *model.CommandConfig) (err error) {
	if len(c.New.SkeletonPath) == 0 {
		c.New.SkeletonPath = "https://" + RevelSkeletonsImportPath + ":basic/bootstrap4"
	}

	// First check to see the protocol of the string
	sp, err := url.Parse(c.New.SkeletonPath)
	if err == nil {
		utils.Logger.Info("Detected skeleton path", "path", sp)

		switch strings.ToLower(sp.Scheme) {
		// TODO Add support for ftp, sftp, scp ??
		case "" :
			sp.Scheme = "file"
			fallthrough
		case "file" :
			fullpath := sp.String()[7:]
			if !filepath.IsAbs(fullpath) {
				fullpath, err = filepath.Abs(fullpath)
				if err != nil {
					return
				}
			}
			c.New.SkeletonPath = fullpath
			utils.Logger.Info("Set skeleton path to ", fullpath)
			if !utils.DirExists(fullpath) {
				return fmt.Errorf("Failed to find skeleton in filepath %s %s", fullpath, sp.String())
			}
		case "git":
			fallthrough
		case "http":
			fallthrough
		case "https":
			if err := newLoadFromGit(c, sp); err != nil {
				return err
			}
		default:
			utils.Logger.Fatal("Unsupported skeleton schema ", "path", c.New.SkeletonPath)

		}
		// TODO check to see if the path needs to be extracted
	} else {
		utils.Logger.Fatal("Invalid skeleton path format", "path", c.New.SkeletonPath)
	}
	return
}

// Load skeleton from git
func newLoadFromGit(c *model.CommandConfig, sp *url.URL) (err error) {
	// This method indicates we need to fetch from a repository using git
	// Execute "git clone get <pkg>"
	targetPath := filepath.Join(os.TempDir(), "revel", "skeleton")
	os.RemoveAll(targetPath)
	pathpart := strings.Split(sp.Path, ":")
	getCmd := exec.Command("git", "clone", sp.Scheme + "://" + sp.Host + pathpart[0], targetPath)
	utils.Logger.Info("Exec:", "args", getCmd.Args)
	getOutput, err := getCmd.CombinedOutput()
	if err != nil {
		utils.Logger.Fatal("Abort: could not clone the  Skeleton  source code: ", "output", string(getOutput), "path", c.New.SkeletonPath)
	}
	outputPath := targetPath
	if len(pathpart) > 1 {
		outputPath = filepath.Join(targetPath, filepath.Join(strings.Split(pathpart[1], string('/'))...))
	}
	outputPath, _ = filepath.Abs(outputPath)
	if !strings.HasPrefix(outputPath, targetPath) {
		utils.Logger.Fatal("Unusual target path outside root path", "target", outputPath, "root", targetPath)
	}

	c.New.SkeletonPath = outputPath
	return
}

func copyNewAppFiles(c *model.CommandConfig) (err error) {
	err = os.MkdirAll(c.AppPath, 0777)
	if err != nil {
		return utils.NewBuildIfError(err, "MKDIR failed")
	}

	err = utils.CopyDir(c.AppPath, c.New.SkeletonPath, map[string]interface{}{
		// app.conf
		"AppName":  c.AppName,
		"BasePath": c.AppPath,
		"Secret":   generateSecret(),
	})
	if err != nil {
		fmt.Printf("err %v", err)
		return utils.NewBuildIfError(err, "Copy Dir failed")
	}

	// Dotfiles are skipped by mustCopyDir, so we have to explicitly copy the .gitignore.
	gitignore := ".gitignore"
	return utils.CopyFile(filepath.Join(c.AppPath, gitignore), filepath.Join(c.New.SkeletonPath, gitignore))

}

const (
	VENDOR_GOPKG = `#
# Revel Gopkg.toml
#
# If you want to use a specific version of Revel change the branches below
#
# Refer to https://github.com/golang/dep/blob/master/docs/Gopkg.toml.md
# for detailed Gopkg.toml documentation.
#
# required = ["github.com/user/thing/cmd/thing"]
# ignored = ["github.com/user/project/pkgX", "bitbucket.org/user/project/pkgA/pkgY"]
#
# [[constraint]]
#   name = "github.com/user/project"
#   version = "1.0.0"
#
# [[constraint]]
#   name = "github.com/user/project2"
#   branch = "dev"
#   source = "github.com/myfork/project2"
#
# [[override]]
#  name = "github.com/x/y"
#  version = "2.4.0"
required = ["github.com/revel/revel", "github.com/revel/modules"]

# Note to use a specific version changes this to
#
# [[override]]
#   version = "0.20.1"
#   name = "github.com/revel/modules"

[[override]]
  branch = "master"
  name = "github.com/revel/modules"

# Note to use a specific version changes this to
#
# [[override]]
#   version = "0.20.0"
#   name = "github.com/revel/revel"
[[override]]
  branch = "master"
  name = "github.com/revel/revel"

[[override]]
  branch = "master"
  name = "github.com/revel/log15"

[[override]]
  branch = "master"
  name = "github.com/revel/cron"

[[override]]
  branch = "master"
  name = "github.com/xeonx/timeago"

`
	NEW_MAIN_FILE = `package main

	`
)
