package gdrive

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/KJHJason/Cultured-Downloader-CLI/configs"
	"github.com/KJHJason/Cultured-Downloader-CLI/request"
	"github.com/KJHJason/Cultured-Downloader-CLI/utils"
	"github.com/fatih/color"
)

const (
	GDRIVE_ERROR_FILENAME = "gdrive_download.log"

	// file fields to fetch from GDrive API:
	// https://developers.google.com/drive/api/v3/reference/files
	GDRIVE_FILE_FIELDS = "id,name,size,mimeType,md5Checksum"
)

type GDrive struct {
	apiKey             string // Google Drive API key to use
	apiUrl             string // https://www.googleapis.com/drive/v3/files
	timeout            int    // timeout in seconds for GDrive API v3
	downloadTimeout    int    // timeout in seconds for GDrive file downloads
	maxDownloadWorkers int    // max concurrent workers for downloading files
}

// Returns a GDrive structure with the given API key and max download workers
func GetNewGDrive(apiKey string, config *configs.Config, maxDownloadWorkers int) *GDrive {
	gdrive := &GDrive{
		apiKey:             apiKey,
		apiUrl:             "https://www.googleapis.com/drive/v3/files",
		timeout:            15,
		downloadTimeout:    900, // 15 minutes
		maxDownloadWorkers: maxDownloadWorkers,
	}

	gdriveIsValid, err := gdrive.GDriveKeyIsValid(config.UserAgent)
	if err != nil {
		color.Red(err.Error())
		os.Exit(1)
	} else if !gdriveIsValid {
		color.Red("Google Drive API key is invalid.")
		os.Exit(1)
	}
	return gdrive
}

// Checks if the given Google Drive API key is valid
//
// Will return true if the given Google Drive API key is valid
func (gdrive *GDrive) GDriveKeyIsValid(userAgent string) (bool, error) {
	match, _ := regexp.MatchString(`^AIza[\w-]{35}$`, gdrive.apiKey)
	if !match {
		return false, nil
	}

	params := map[string]string{"key": gdrive.apiKey}
	res, err := request.CallRequest(
		&request.RequestArgs{
			Url:       gdrive.apiUrl,
			Method:    "GET",
			Timeout:   gdrive.timeout,
			Params:    params,
			UserAgent: userAgent,
		},
	)
	if err != nil {
		err = fmt.Errorf(
			"gdrive error %d: failed to check if Google Drive API key is valid, more info => %v",
			utils.CONNECTION_ERROR,
			err,
		)
		return false, err
	}
	res.Body.Close()
	return res.StatusCode != 400, nil
}

// Logs any failed GDrive API calls to the given log path
func LogFailedGdriveAPICalls(res *http.Response, downloadPath string) {
	requestUrl := res.Request.URL.String()
	errorMsg := fmt.Sprintf(
		"Error while fetching from GDrive...\n"+
			"GDrive URL (May not be accurate): https://drive.google.com/file/d/%s/view?usp=sharing\n"+
			"Status Code: %s\nURL: %s",
		utils.GetLastPartOfUrl(requestUrl),
		res.Status,
		requestUrl,
	)
	if downloadPath != "" {
		utils.LogError(nil, errorMsg, false)
		return
	}

	// create new text file
	var logFilePath string
	logFilename := GDRIVE_ERROR_FILENAME
	if filepath.Ext(downloadPath) == "" {
		logFilePath = filepath.Join(filepath.Dir(downloadPath), logFilename)
	} else {
		logFilePath = filepath.Join(downloadPath, logFilename)
	}
	file, err := os.OpenFile(
		logFilePath,
		os.O_WRONLY|os.O_CREATE|os.O_APPEND,
		0666,
	)
	if err != nil {
		err = fmt.Errorf(
			"gdrive error %d: failed to open log file, more info => %v",
			utils.OS_ERROR,
			err,
		)
		utils.LogError(err, "", false)
		return
	}
	defer file.Close()

	// write to file
	_, err = file.WriteString(errorMsg)
	if err != nil {
		err = fmt.Errorf(
			"gdrive error %d: failed to write to log file, more info => %v",
			utils.OS_ERROR,
			err,
		)
		utils.LogError(err, "", false)
	}
}

type GDriveFile struct {
	Kind        string `json:"kind"`
	Id          string `json:"id"`
	Name        string `json:"name"`
	Size        string `json:"size"`
	MimeType    string `json:"mimeType"`
	Md5Checksum string `json:"md5Checksum"`
}

type GDriveFolder struct {
	Kind             string       `json:"kind"`
	IncompleteSearch bool         `json:"incompleteSearch"`
	Files            []GDriveFile `json:"files"`
	NextPageToken    string       `json:"nextPageToken"`
}

// Returns the contents of the given GDrive folder
func (gdrive *GDrive) GetFolderContents(folderId, logPath string, config *configs.Config) ([]map[string]string, error) {
	params := map[string]string{
		"key":    gdrive.apiKey,
		"q":      fmt.Sprintf("'%s' in parents", folderId),
		"fields": fmt.Sprintf("nextPageToken,files(%s)", GDRIVE_FILE_FIELDS),
	}
	files := []map[string]string{}
	pageToken := ""
	for {
		if pageToken != "" {
			params["pageToken"] = pageToken
		} else {
			delete(params, "pageToken")
		}
		res, err := request.CallRequest(
			&request.RequestArgs{
				Url:       gdrive.apiUrl,
				Method:    "GET",
				Timeout:   gdrive.timeout,
				Params:    params,
				UserAgent: config.UserAgent,
			},
		)
		if err != nil {
			err = fmt.Errorf(
				"gdrive error %d: failed to get folder contents with ID of %s, more info => %v",
				utils.CONNECTION_ERROR,
				folderId,
				err,
			)
			return nil, err
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			err = fmt.Errorf(
				"gdrive error %d: failed to get folder contents with ID of %s, more info => %s",
				utils.RESPONSE_ERROR,
				folderId,
				res.Status,
			)
			return nil, err
		}

		gdriveRes, err := utils.ReadResBody(res)
		if err != nil {
			return nil, err
		}

		gdriveFolder := GDriveFolder{}
		if err := json.Unmarshal(gdriveRes, &gdriveFolder); err != nil {
			err = fmt.Errorf(
				"gdrive error %d: failed to unmarshal GDrive folder contents with ID of %s, more info => %v\nResponse body: %s",
				utils.JSON_ERROR,
				folderId,
				err,
				string(gdriveRes),
			)
			return nil, err
		}
		for _, file := range gdriveFolder.Files {
			files = append(files, map[string]string{
				"id":          file.Id,
				"name":        file.Name,
				"size":        file.Size,
				"mimeType":    file.MimeType,
				"md5Checksum": file.Md5Checksum,
			})
		}

		if gdriveFolder.NextPageToken == "" {
			break
		} else {
			pageToken = gdriveFolder.NextPageToken
		}
	}
	return files, nil
}

// Retrieves the content of a GDrive folder and its subfolders recursively using GDrive API v3
func (gdrive *GDrive) GetNestedFolderContents(folderId, logPath string, config *configs.Config) ([]map[string]string, error) {
	files := []map[string]string{}
	folderContents, err := gdrive.GetFolderContents(folderId, logPath, config)
	if err != nil {
		return nil, err
	}

	for _, file := range folderContents {
		if file["mimeType"] == "application/vnd.google-apps.folder" {
			subFolderFiles, err := gdrive.GetNestedFolderContents(file["id"], logPath, config)
			if err != nil {
				return nil, err
			}
			files = append(files, subFolderFiles...)
		} else {
			files = append(files, file)
		}
	}
	return files, nil
}

// Retrieves the file details of the given GDrive file using GDrive API v3
func (gdrive *GDrive) GetFileDetails(fileId, logPath string, config *configs.Config) (map[string]string, error) {
	params := map[string]string{
		"key":    gdrive.apiKey,
		"fields": GDRIVE_FILE_FIELDS,
	}
	url := fmt.Sprintf("%s/%s", gdrive.apiUrl, fileId)
	res, err := request.CallRequest(
		&request.RequestArgs{
			Url:       url,
			Method:    "GET",
			Timeout:   gdrive.timeout,
			Params:    params,
			UserAgent: config.UserAgent,
		},
	)
	if err != nil {
		err = fmt.Errorf(
			"gdrive error %d: failed to get file details with ID of %s, more info => %v",
			utils.CONNECTION_ERROR,
			fileId,
			err,
		)
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		LogFailedGdriveAPICalls(res, logPath)
		return nil, nil
	}

	resBody, err := utils.ReadResBody(res)
	if err != nil {
		return nil, err
	}

	gdriveFile := GDriveFile{}
	if err := json.Unmarshal(resBody, &gdriveFile); err != nil {
		err = fmt.Errorf(
			"gdrive error %d: failed to unmarshal GDrive file details with ID of %s, more info => %s\nResponse body: %v",
			utils.JSON_ERROR,
			fileId,
			err,
			string(resBody),
		)
		return nil, err
	}
	return map[string]string{
		"id":          gdriveFile.Id,
		"name":        gdriveFile.Name,
		"size":        gdriveFile.Size,
		"mimeType":    gdriveFile.MimeType,
		"md5Checksum": gdriveFile.Md5Checksum,
	}, nil
}
