package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/fatih/color"
)

// checks if a file or directory exists
func PathExists(filepath string) bool {
	_, err := os.Stat(filepath)
	return !os.IsNotExist(err)
}

// Returns the file size based on the provided file path
//
// If the file does not exist or
// there was an error opening the file at the given file path string, -1 is returned
	if PathExists(filePath) {
		}
		fileInfo, err := file.Stat()
		if err != nil {
			return -1, err
		}
		return fileInfo.Size(), nil
	}
	return -1, nil
}

var logToPathMutex sync.Mutex

// Thread-safe logging function that logs to the provided file path
func LogMessageToPath(message, filePath string) {
	logToPathMutex.Lock()
	defer logToPathMutex.Unlock()

	os.MkdirAll(filepath.Dir(filePath), 0666)
	logFile, err := os.OpenFile(
		filePath,
		os.O_WRONLY|os.O_CREATE|os.O_APPEND,
		0666,
	)
	if err != nil {
		errMsg := fmt.Sprintf(
			"error %d: failed to open log file, more info => %v\nfile path: %s\noriginal message: %s",
			OS_ERROR,
			err,
			filePath,
			message,
		)
		color.Red(errMsg)
		return
	}
	defer logFile.Close()

	_, err = logFile.WriteString(message)
	if err != nil {
		errMsg := fmt.Sprintf(
			"error %d: failed to write to log file, more info => %v\nfile path: %s\noriginal message: %s",
			OS_ERROR,
			err,
			filePath,
			message,
		)
		color.Red(errMsg)
	}
}

// Removes any illegal characters in a path name
// to prevent any error with file I/O using the path name
func RemoveIllegalCharsInPathName(dirtyPathName string) string {
	dirtyPathName = strings.TrimSpace(dirtyPathName)
	partiallyCleanedPathName := strings.ReplaceAll(dirtyPathName, ".", " ")
	return ILLEGAL_PATH_CHARS_REGEX.ReplaceAllString(partiallyCleanedPathName, "-")
}

// Returns a directory path for a post, artwork, etc.
// based on the user's saved download path and the provided arguments
func GetPostFolder(downloadPath, creatorName, postId, postTitle string) string {
	creatorName = RemoveIllegalCharsInPathName(creatorName)
	postTitle = RemoveIllegalCharsInPathName(postTitle)

	postFolderPath := filepath.Join(
		downloadPath,
		creatorName,
		fmt.Sprintf("[%s] %s", postId, postTitle),
	)
	return postFolderPath
}

type ConfigFile struct {
	DownloadDir        string `json:"download_directory"`
	Language           string `json:"language"`
	ClientDigestMethod string `json:"client_digest_method"`
}

// Returns the download path from the config file
func GetDefaultDownloadPath() string {
	configFilePath := filepath.Join(APP_PATH, "config.json")
	if !PathExists(configFilePath) {
		return ""
	}

	configFile, err := os.ReadFile(configFilePath)
	if err != nil {
		os.Remove(configFilePath)
		return ""
	}

	var config ConfigFile
	err = json.Unmarshal(configFile, &config)
	if err != nil {
		os.Remove(configFilePath)
		return ""
	}

	if !PathExists(config.DownloadDir) {
		return ""
	}
	return config.DownloadDir
}

// Pretify a JSON bytes input by indenting it with 4 whitespaces
func PretifyJSON(jsonBytes []byte) ([]byte, error) {
	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, jsonBytes, "", "    ")
	if err != nil {
		return []byte{}, err
	}
	return prettyJSON.Bytes(), nil
}

// Configure and saves the config file with updated download path
func SetDefaultDownloadPath(newDownloadPath string) error {
	if !PathExists(newDownloadPath) {
		return fmt.Errorf("error %d: download path does not exist, please create the directory and try again", INPUT_ERROR)
	}

	os.MkdirAll(APP_PATH, 0666)
	configFilePath := filepath.Join(APP_PATH, "config.json")
	if !PathExists(configFilePath) {
		os.Create(configFilePath)

		is64Bit := strconv.IntSize == 64
		digestMethod := "sha256"
		if is64Bit {
			digestMethod = "sha512"
		}
		config := ConfigFile{
			DownloadDir:        newDownloadPath,
			Language:           "en",
			ClientDigestMethod: digestMethod,
		}

		configFile, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf(
				"error %d: failed to marshal config file, more info => %v",
				JSON_ERROR,
				err,
			)
		}

		configFile, err = PretifyJSON(configFile)
		if err != nil {
			return fmt.Errorf(
				"error %d: failed to pretify config file, more info => %v",
				JSON_ERROR,
				err,
			)
		}

		err = os.WriteFile(configFilePath, configFile, 0666)
		if err != nil {
			return fmt.Errorf(
				"error %d: failed to write config file, more info => %v",
				OS_ERROR,
				err,
			)
		}
	} else {
		// read the file
		configFile, err := os.ReadFile(configFilePath)
		if err != nil {
			return fmt.Errorf(
				"error %d: failed to read config file, more info => %v",
				OS_ERROR,
				err,
			)
		}

		var config ConfigFile
		err = json.Unmarshal(configFile, &config)
		if err != nil {
			return fmt.Errorf(
				"error %d: failed to unmarshal config file, more info => %v",
				JSON_ERROR,
				err,
			)
		}

		// update the file if the download directory is different
		if config.DownloadDir == newDownloadPath {
			return nil
		}

		config.DownloadDir = newDownloadPath
		configFile, err = json.Marshal(config)
		if err != nil {
			return fmt.Errorf(
				"error %d: failed to marshal config file, more info => %v",
				JSON_ERROR,
				err,
			)
		}

		// indent the file
		configFile, err = PretifyJSON(configFile)
		if err != nil {
			return fmt.Errorf(
				"error %d: failed to pretify config file, more info => %v",
				JSON_ERROR,
				err,
			)
		}

		err = os.WriteFile(configFilePath, configFile, 0666)
		if err != nil {
			return fmt.Errorf(
				"error %d: failed to write config file, more info => %v",
				OS_ERROR,
				err,
			)
		}
	}
	return nil
}
