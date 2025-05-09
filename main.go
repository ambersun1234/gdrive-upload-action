// Mathis Van Eetvelde
// 2021-present

package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sethvargo/go-githubactions"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
)

const (
	scope            = "https://www.googleapis.com/auth/drive.file"
	filenameInput    = "filename"
	nameInput        = "name"
	folderIdInput    = "folderId"
	credentialsInput = "credentials"
	encodedInput     = "encoded"
	overwrite        = "overwrite"
)

func main() {

	// get filename argument from action input
	filename := githubactions.GetInput(filenameInput)
	if filename == "" {
		missingInput(filenameInput)
	}

	// get name argument from action input
	name := githubactions.GetInput(nameInput)

	// get folderId argument from action input
	folderId := githubactions.GetInput(folderIdInput)
	if folderId == "" {
		missingInput(folderIdInput)
	}

	// get base64 encoded credentials argument from action input
	credentialsStr := githubactions.GetInput(credentialsInput)
	if credentialsStr == "" {
		missingInput(credentialsInput)
	}
	// add base64 encoded credentials argument to mask
	githubactions.AddMask(credentialsStr)

	// get encoded boolean input
	var encoded bool
	encodedStr := githubactions.GetInput(encodedInput)
	if encodedStr == "" || encodedStr == "true" {
		encoded = true
	} else if encodedStr == "false" {
		encoded = false
	} else {
		incorrectInput(encodedInput, "encoded needs to be either empty, `false` or `true`.")
	}

	// decode if encoded is true
	var credentials string
	if encoded {
		// decode credentials to []byte
		credentialsByte, err := base64.StdEncoding.DecodeString(credentialsStr)
		if err != nil {
			githubactions.Fatalf(fmt.Sprintf("base64 decoding of 'credentials' failed with error: %v", err))
		}

		credentials = string(credentialsByte)
	} else {
		credentials = credentialsStr
	}

	creds := strings.TrimSuffix(string(credentials), "\n")

	// add decoded credentials argument to mask
	githubactions.AddMask(creds)

	// fetching a JWT config with credentials and the right scope
	conf, err := google.JWTConfigFromJSON([]byte(creds), scope)
	if err != nil {
		githubactions.Fatalf(fmt.Sprintf("fetching JWT credentials failed with error: %v", err))
	}

	// instantiating a new drive service
	ctx := context.Background()
	svc, err := drive.New(conf.Client(ctx))
	if err != nil {
		log.Println(err)
	}

	file, err := os.Open(filename)
	if err != nil {
		githubactions.Fatalf(fmt.Sprintf("opening file with filename: %v failed with error: %v", filename, err))
	}

	// decide name of file in GDrive
	if name == "" {
		name = file.Name()
	}

	// parse overwrite flag
	overwriteStr := githubactions.GetInput(overwrite)
	var overwrite bool = false
	if overwriteStr == "true" {
		overwrite = true
	}

	switch overwrite {
	case true:
		filenameQuery := fmt.Sprintf("name = '%s' and '%s' in parents", name, folderId)
		filesQueryCallResult, err := svc.Files.
			List().
			IncludeItemsFromAllDrives(true).
			SupportsAllDrives(true).
			Q(filenameQuery).
			Do()
		if err != nil {
			githubactions.Fatalf(fmt.Sprintf("failed to query name %s in %s", name, folderId))
		}

		size := len(filesQueryCallResult.Files)
		if size == 0 {
			githubactions.Fatalf(fmt.Sprintf("cannot find %s in %s", name, folderId))
		}
		if size > 1 {
			githubactions.Fatalf("over 1 file matched the query condition")
		}

		githubactions.Infof(fmt.Sprintf("found %v matched files under %s", size, folderId))

		// overwrite file content
		driveFile := filesQueryCallResult.Files[0]
		_, err = svc.Files.
			Update(driveFile.Id, &drive.File{Name: name}).
			SupportsAllDrives(true).
			Media(file).
			Do()
		if err != nil {
			githubactions.Fatalf(fmt.Sprintf("updating file: %+v failed with error: %v", driveFile, err))
		}
		githubactions.Infof(
			"Updating file %s (in folder %s) with id %s", driveFile.Name, folderId, driveFile.Id,
		)

	default:
		f := &drive.File{
			Name:    name,
			Parents: []string{folderId},
		}
		if _, err := svc.Files.Create(f).Media(file).SupportsAllDrives(true).Do(); err != nil {
			githubactions.Fatalf(fmt.Sprintf("creating file: %+v failed with error: %v", f, err))
		}
		githubactions.Infof("Creating file %s in folder %s", f.Name, folderId)
	}
}

func missingInput(inputName string) {
	githubactions.Fatalf(fmt.Sprintf("missing input '%v'", inputName))
}

func incorrectInput(inputName string, reason string) {
	if reason == "" {
		githubactions.Fatalf(fmt.Sprintf("incorrect input '%v'", inputName))
	} else {
		githubactions.Fatalf(fmt.Sprintf("incorrect input '%v' reason: %v", inputName, reason))
	}
}
