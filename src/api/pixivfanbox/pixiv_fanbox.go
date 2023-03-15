package pixivfanbox

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/KJHJason/Cultured-Downloader-CLI/configs"
	"github.com/KJHJason/Cultured-Downloader-CLI/api/pixivfanbox/models"
	"github.com/KJHJason/Cultured-Downloader-CLI/request"
	"github.com/KJHJason/Cultured-Downloader-CLI/spinner"
	"github.com/KJHJason/Cultured-Downloader-CLI/utils"
)

// Returns a defined request header needed to communicate with Pixiv Fanbox's API
func GetPixivFanboxHeaders() map[string]string {
	return map[string]string{
		"Origin":  utils.PIXIV_FANBOX_URL,
		"Referer": utils.PIXIV_FANBOX_URL,
	}
}

// Process and detects for any external download links from the post's text content
func processPixivFanboxText(postBodyStr, postFolderPath string, downloadGdrive bool) []map[string]string {
	if postBodyStr == "" {
		return nil
	}

	// split the text by newlines
	postBodySlice := strings.FieldsFunc(
		postBodyStr,
		func(c rune) bool {
			return c == '\n'
		},
	)
	loggedPassword := false
	var detectedGdriveLinks []map[string]string
	for _, text := range postBodySlice {
		if utils.DetectPasswordInText(text) && !loggedPassword {
			// Log the entire post text if it contains a password
			filePath := filepath.Join(postFolderPath, utils.PASSWORD_FILENAME)
			if !utils.PathExists(filePath) {
				loggedPassword = true
				postBodyStr := strings.Join(postBodySlice, "\n")
				utils.LogMessageToPath(
					"Found potential password in the post:\n\n"+postBodyStr,
					filePath,
				)
			}
		}

		utils.DetectOtherExtDLLink(text, postFolderPath)
		if utils.DetectGDriveLinks(text, postFolderPath, false) && downloadGdrive {
			detectedGdriveLinks = append(detectedGdriveLinks, map[string]string{
				"url":      text,
				"filepath": filepath.Join(postFolderPath, utils.GDRIVE_FOLDER),
			})
		}
	}
	return detectedGdriveLinks
}

// Pixiv Fanbox permitted file extensions based on
// https://fanbox.pixiv.help/hc/en-us/articles/360011057793-What-types-of-attachments-can-I-post-
var pixivFanboxAllowedImageExt = []string{"jpg", "jpeg", "png", "gif"}

// Process the JSON response from Pixiv Fanbox's API and
// returns a map of urls and a map of GDrive urls to download from
func processFanboxPost(res *http.Response, downloadPath string, pixivFanboxDlOptions *PixivFanboxDlOptions) ([]map[string]string, []map[string]string, error) {
	var err error
	var post models.FanboxPostJson
	var postJsonBody []byte
	err = utils.LoadJsonFromResponse(res, &post)
	if err != nil {
		return nil, nil, err
	}

	postJson := post.Body
	postId := postJson.Id
	postTitle := postJson.Title
	creatorId := postJson.CreatorId
	postFolderPath := utils.GetPostFolder(
		filepath.Join(downloadPath, "Pixiv-Fanbox"),
		creatorId,
		postId,
		postTitle,
	)

	var urlsMap []map[string]string
	thumbnail := postJson.CoverImageUrl
	if pixivFanboxDlOptions.DlThumbnails && thumbnail != "" {
		urlsMap = append(urlsMap, map[string]string{
			"url":      thumbnail,
			"filepath": postFolderPath,
		})
	}

	// Note that Pixiv Fanbox posts have 3 types of formatting (as of now):
	//	1. With proper formatting and mapping of post content elements ("article")
	//	2. With a simple formatting that obly contains info about the text and files ("file", "image")
	postType := postJson.Type
	postBody := postJson.Body
	if postBody == nil {
		return urlsMap, nil, nil
	}

	var gdriveLinks []map[string]string
	switch postType {
	case "file":
		// process the text in the post
		filePostJson := postBody.(*models.FanboxFilePostJson)
		detectedGdriveLinks := processPixivFanboxText(
			filePostJson.Text,
			postFolderPath,
			pixivFanboxDlOptions.DlGdrive,
		)
		if detectedGdriveLinks != nil {
			gdriveLinks = append(gdriveLinks, detectedGdriveLinks...)
		}

		imageAndAttachmentUrls := filePostJson.Files
		if (len(imageAndAttachmentUrls) > 0) && (pixivFanboxDlOptions.DlImages || pixivFanboxDlOptions.DlAttachments) {
			for _, fileInfo := range imageAndAttachmentUrls {
				fileUrl := fileInfo.Url
				extension := fileInfo.Extension
				filename := fileInfo.Name + "." + extension

				var filePath string
				isImage := utils.SliceContains(pixivFanboxAllowedImageExt, extension)
				if isImage {
					filePath = filepath.Join(postFolderPath, utils.IMAGES_FOLDER, filename)
				} else {
					filePath = filepath.Join(postFolderPath, utils.ATTACHMENT_FOLDER, filename)
				}

				if (isImage && pixivFanboxDlOptions.DlImages) || (!isImage && pixivFanboxDlOptions.DlAttachments) {
					urlsMap = append(urlsMap, map[string]string{
						"url":      fileUrl,
						"filepath": filePath,
					})
				}
			}
		}
	case "image":
		// process the text in the post
		imagePostJson := postBody.(*models.FanboxImagePostJson)
		detectedGdriveLinks := processPixivFanboxText(
			imagePostJson.Text,
			postFolderPath,
			pixivFanboxDlOptions.DlGdrive,
		)
		if detectedGdriveLinks != nil {
			gdriveLinks = append(gdriveLinks, detectedGdriveLinks...)
		}

		// retrieve images and attachments url(s)
		imageAndAttachmentUrls := imagePostJson.Images
		if (imageAndAttachmentUrls != nil) && (pixivFanboxDlOptions.DlImages || pixivFanboxDlOptions.DlAttachments) {
			for _, fileInfo := range imageAndAttachmentUrls {
				fileUrl := fileInfo.OriginalUrl
				extension := fileInfo.Extension
				filename := utils.GetLastPartOfUrl(fileUrl)

				var filePath string
				isImage := utils.SliceContains(pixivFanboxAllowedImageExt, extension)
				if isImage {
					filePath = filepath.Join(postFolderPath, utils.IMAGES_FOLDER, filename)
				} else {
					filePath = filepath.Join(postFolderPath, utils.ATTACHMENT_FOLDER, filename)
				}

				if (isImage && pixivFanboxDlOptions.DlImages) || (!isImage && pixivFanboxDlOptions.DlAttachments) {
					urlsMap = append(urlsMap, map[string]string{
						"url":      fileUrl,
						"filepath": filePath,
					})
				}
			}
		}
	case "article":
		// process the text in the post
		articleJson := postBody.(*models.FanboxArticleJson)
		articleBlocks := articleJson.Blocks
		if len(articleBlocks) > 0 {
			loggedPassword := false
			for _, articleBlock := range articleBlocks {
				text := articleBlock.Text
				if text != "" {
					if utils.DetectPasswordInText(text) && !loggedPassword {
						// Log the entire post text if it contains a password
						filePath := filepath.Join(postFolderPath, utils.PASSWORD_FILENAME)
						if !utils.PathExists(filePath) {
							loggedPassword = true
							postBodyStr := "Found potential password in the post:\n\n"
							for _, articleContent := range articleBlocks {
								articleText := articleContent.Text
								if articleText != "" {
									postBodyStr += articleText + "\n"
								}
							}
							utils.LogMessageToPath(
								postBodyStr,
								filePath,
							)
						}
					}

					utils.DetectOtherExtDLLink(text, postFolderPath)
					if utils.DetectGDriveLinks(text, postFolderPath, false) && pixivFanboxDlOptions.DlGdrive {
						gdriveLinks = append(gdriveLinks, map[string]string{
							"url":      text,
							"filepath": filepath.Join(postFolderPath, utils.GDRIVE_FOLDER),
						})
					}
				}

				articleLinks := articleBlock.Links
				if len(articleLinks) > 0 {
					for _, articleLink := range articleLinks {
						linkUrl := articleLink.Url
						utils.DetectOtherExtDLLink(linkUrl, postFolderPath)
						if utils.DetectGDriveLinks(linkUrl, postFolderPath, true) && pixivFanboxDlOptions.DlGdrive {
							gdriveLinks = append(gdriveLinks, map[string]string{
								"url":      linkUrl,
								"filepath": filepath.Join(postFolderPath, utils.GDRIVE_FOLDER),
							})
							continue
						}
					}
				}
			}
		}

		// retrieve images and attachments url(s)
		imageMap := articleJson.ImageMap
		if imageMap != nil && pixivFanboxDlOptions.DlImages {
			for _, imageInfo := range imageMap {
				urlsMap = append(urlsMap, map[string]string{
					"url":      imageInfo.OriginalUrl,
					"filepath": filepath.Join(postFolderPath, utils.IMAGES_FOLDER),
				})
			}
		}

		attachmentMap := articleJson.FileMap
		if attachmentMap != nil && pixivFanboxDlOptions.DlAttachments {
			for _, attachmentInfo := range attachmentMap {
				attachmentUrl := attachmentInfo.Url
				filename := attachmentInfo.Name + "." + attachmentInfo.Extension
				urlsMap = append(urlsMap, map[string]string{
					"url":      attachmentUrl,
					"filepath": filepath.Join(postFolderPath, utils.ATTACHMENT_FOLDER, filename),
				})
			}
		}
	case "text": // text post
		// Usually has no content but try to detect for any external download links
		textContent := postBody.(*models.FanboxTextPostJson)
		detectedGdriveLinks := processPixivFanboxText(
			textContent.Text,
			postFolderPath,
			pixivFanboxDlOptions.DlGdrive,
		)
		if detectedGdriveLinks != nil {
			gdriveLinks = append(gdriveLinks, detectedGdriveLinks...)
		}
	default: // unknown post type
		return nil, nil, fmt.Errorf(
			"pixiv fanbox error %d: unknown post type, \"%s\"\nPixiv Fanbox post content:\n%s",
			utils.JSON_ERROR,
			postType,
			string(postJsonBody),
		)
	}
	return urlsMap, gdriveLinks, nil
}

// Query Pixiv Fanbox's API based on the slice of post IDs and returns a map of
// urls and a map of GDrive urls to download from
func getPostDetails(postIds []string, config *configs.Config, pixivFanboxDlOptions *PixivFanboxDlOptions) ([]map[string]string, []map[string]string) {
	maxConcurrency := utils.MAX_API_CALLS
	postIdsLen := len(postIds)
	if postIdsLen < maxConcurrency {
		maxConcurrency = postIdsLen
	}
	var wg sync.WaitGroup
	queue := make(chan struct{}, maxConcurrency)
	resChan := make(chan *http.Response, postIdsLen)
	errChan := make(chan error, postIdsLen)

	baseMsg := "Getting post details from Pixiv Fanbox [%d/" + fmt.Sprintf("%d]...", postIdsLen)
	progress := spinner.New(
		spinner.REQ_SPINNER,
		"fgHiYellow",
		fmt.Sprintf(
			baseMsg,
			0,
		),
		fmt.Sprintf(
			"Finished getting %d post details from Pixiv Fanbox!",
			postIdsLen,
		),
		fmt.Sprintf(
			"Something went wrong while getting %d post details from Pixiv Fanbox.\nPlease refer to the logs for more details.",
			postIdsLen,
		),
		postIdsLen,
	)
	progress.Start()

	url := fmt.Sprintf("%s/post.info", utils.PIXIV_FANBOX_API_URL)
	for _, postId := range postIds {
		wg.Add(1)
		queue <- struct{}{}
		go func(postId string) {
			defer func() {
				<-queue
				wg.Done()
			}()

			header := GetPixivFanboxHeaders()
			params := map[string]string{"postId": postId}
			res, err := request.CallRequest(
				&request.RequestArgs{
					Method:    "GET",
					Url:       url,
					Cookies:   pixivFanboxDlOptions.SessionCookies,
					Headers:   header,
					Params:    params,
					UserAgent: config.UserAgent,
				},
			)
			if err != nil {
				errChan <- fmt.Errorf(
					"pixiv fanbox error %d: failed to get post details for %s, more info => %v",
					utils.CONNECTION_ERROR,
					url,
					err,
				)
			} else if res.StatusCode != 200 {
				errChan <- fmt.Errorf(
					"pixiv fanbox error %d: failed to get post details for %s due to a %s response",
					utils.CONNECTION_ERROR,
					url,
					res.Status,
				)
			} else {
				resChan <- res
			}
			progress.MsgIncrement(baseMsg)
		}(postId)
	}
	close(queue)
	wg.Wait()
	close(resChan)
	close(errChan)

	hasErr := false
	if len(errChan) > 0 {
		hasErr = true
		utils.LogErrors(false, errChan)
	}
	progress.Stop(hasErr)

	// parse the responses
	var errSlice []error
	var urlsMap, gdriveUrls []map[string]string
	baseMsg = "Processing received JSON(s) from Pixiv Fanbox [%d/" + fmt.Sprintf("%d]...", len(resChan))
	progress = spinner.New(
		spinner.JSON_SPINNER,
		"fgHiYellow",
		fmt.Sprintf(
			baseMsg,
			0,
		),
		fmt.Sprintf(
			"Finished processing %d JSON(s) from Pixiv Fanbox!",
			len(resChan),
		),
		fmt.Sprintf(
			"Something went wrong while processing %d JSON(s) from Pixiv Fanbox.\nPlease refer to the logs for more details.",
			len(resChan),
		),
		len(resChan),
	)
	progress.Start()
	for res := range resChan {
		postUrls, postGdriveLinks, err := processFanboxPost(
			res,
			utils.DOWNLOAD_PATH,
			pixivFanboxDlOptions,
		)
		if err != nil {
			errSlice = append(errSlice, err)
		} else {
			urlsMap = append(urlsMap, postUrls...)
			gdriveUrls = append(gdriveUrls, postGdriveLinks...)
		}
		progress.MsgIncrement(baseMsg)
	}

	hasErr = false
	if len(errSlice) > 0 {
		hasErr = true
		utils.LogErrors(false, nil, errSlice...)
	}
	progress.Stop(hasErr)

	return urlsMap, gdriveUrls
}

type CreatorPaginatedPosts struct {
	Body []string `json:"body"`
}

type FanboxCreatorPosts struct {
	Body struct {
		Items []struct {
			Id string `json:"id"`
		} `json:"items"`
	} `json:"body"`
}

// GetFanboxCreatorPosts returns a slice of post IDs for a given creator
func getFanboxPosts(creatorId, pageNum string, config *configs.Config, cookies []*http.Cookie) ([]string, error) {
	params := map[string]string{"creatorId": creatorId}
	headers := GetPixivFanboxHeaders()
	url := fmt.Sprintf(
		"%s/post.paginateCreator",
		utils.PIXIV_FANBOX_API_URL,
	)
	res, err := request.CallRequest(
		&request.RequestArgs{
			Method:    "GET",
			Url:       url,
			Cookies:   cookies,
			Headers:   headers,
			Params:    params,
			UserAgent: config.UserAgent,
		},
	)
	if err != nil || res.StatusCode != 200 {
		const errPrefix = "pixiv fanbox error"
		if err != nil {
			err = fmt.Errorf(
				"%s %d: failed to get creator's posts for %s due to %v",
				errPrefix,
				utils.CONNECTION_ERROR,
				creatorId,
				err,
			)
		} else {
			res.Body.Close()
			err = fmt.Errorf(
				"%s %d: failed to get creator's posts for %s due to %s response",
				errPrefix,
				utils.RESPONSE_ERROR,
				creatorId,
				res.Status,
			)
		}
		return nil, err
	}

	var resJson CreatorPaginatedPosts
	resBody, err := utils.ReadResBody(res)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		err = fmt.Errorf(
			"pixiv fanbox error %d: failed to unmarshal json for Pixiv Fanbox creator's pages for %s\nJSON: %s",
			utils.JSON_ERROR,
			creatorId,
			string(resBody),
		)
		return nil, err
	}
	paginatedUrls := resJson.Body

	minPage, maxPage, hasMax, err := utils.GetMinMaxFromStr(pageNum)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	maxConcurrency := utils.MAX_API_CALLS
	if len(paginatedUrls) < maxConcurrency {
		maxConcurrency = len(paginatedUrls)
	}
	queue := make(chan struct{}, maxConcurrency)
	resChan := make(chan *http.Response, len(paginatedUrls))
	for idx, paginatedUrl := range paginatedUrls {
		curPage := idx + 1
		if curPage < minPage {
			continue
		}
		if hasMax && curPage > maxPage {
			break
		}

		wg.Add(1)
		queue <- struct{}{}
		go func(reqUrl string) {
			defer wg.Done()
			res, err := request.CallRequest(
				&request.RequestArgs{
					Method:    "GET",
					Url:       reqUrl,
					Cookies:   cookies,
					Headers:   headers,
					UserAgent: config.UserAgent,
				},
			)
			if err != nil || res.StatusCode != 200 {
				if err == nil {
					res.Body.Close()
				}
				utils.LogError(err, fmt.Sprintf("failed to get post for %s", reqUrl), false)
			} else {
				resChan <- res
			}
			<-queue
		}(paginatedUrl)
	}
	close(queue)
	wg.Wait()
	close(resChan)

	// parse the JSON response
	var errSlice []error
	var postIds []string
	for res := range resChan {
		resBody, err := utils.ReadResBody(res)
		if err != nil {
			errSlice = append(errSlice, err)
			continue
		}

		var resJson FanboxCreatorPosts
		err = json.Unmarshal(resBody, &resJson)
		if err != nil {
			err = fmt.Errorf(
				"pixiv fanbox error %d: failed to unmarshal json for Pixiv Fanbox creator's post\nJSON: %s",
				utils.JSON_ERROR,
				string(resBody),
			)
			errSlice = append(errSlice, err)
			continue
		}

		for _, postInfoMap := range resJson.Body.Items {
			postIds = append(postIds, postInfoMap.Id)
		}
	}

	utils.LogErrors(false, nil, errSlice...)
	return postIds, nil
}

// Retrieves all the posts based on the slice of creator IDs and returns a slice of post IDs
func getCreatorsPosts(creatorIds, pageNums []string, config *configs.Config, cookies []*http.Cookie) []string {
	creatorIdsLen := len(creatorIds)
	if creatorIdsLen != len(pageNums) {
		panic(
			fmt.Errorf(
				"pixiv fanbox error %d: length of creator IDs and page numbers are not equal",
				utils.DEV_ERROR,
			),
		)
	}

	var postIds []string
	var errSlice []error
	baseMsg := "Getting post ID(s) from creator(s) on Pixiv Fanbox [%d/" + fmt.Sprintf("%d]...", creatorIdsLen)
	progress := spinner.New(
		spinner.REQ_SPINNER,
		"fgHiYellow",
		fmt.Sprintf(
			baseMsg,
			0,
		),
		fmt.Sprintf(
			"Finished getting post ID(s) from %d creator(s) on Pixiv Fanbox!",
			creatorIdsLen,
		),
		fmt.Sprintf(
			"Something went wrong while getting post IDs from %d creator(s) on Pixiv Fanbox!\nPlease refer to logs for more details.",
			creatorIdsLen,
		),
		creatorIdsLen,
	)
	progress.Start()
	for idx, creatorId := range creatorIds {
		retrievedPostIds, err := getFanboxPosts(
			creatorId,
			pageNums[idx],
			config,
			cookies,
		)
		if err != nil {
			errSlice = append(errSlice, err)
		} else {
			postIds = append(postIds, retrievedPostIds...)
		}
		progress.MsgIncrement(baseMsg)
	}

	hasErr := false
	if len(errSlice) > 0 {
		hasErr = true
		utils.LogErrors(false, nil, errSlice...)
	}
	progress.Stop(hasErr)

	return postIds
}

// Start the download process for Pixiv Fanbox
func PixivFanboxDownloadProcess(config *configs.Config, pixivFanboxDl *PixivFanboxDl, pixivFanboxDlOptions *PixivFanboxDlOptions) {
	if !pixivFanboxDlOptions.DlThumbnails && !pixivFanboxDlOptions.DlImages && !pixivFanboxDlOptions.DlAttachments && !pixivFanboxDlOptions.DlGdrive {
		return
	}

	var urlsToDownload, gdriveUrlsToDownload []map[string]string
	if len(pixivFanboxDl.PostIds) > 0 {
		urlsSlice, gdriveSlice := getPostDetails(
			pixivFanboxDl.PostIds,
			config,
			pixivFanboxDlOptions,
		)
		urlsToDownload = append(urlsToDownload, urlsSlice...)
		gdriveUrlsToDownload = append(gdriveUrlsToDownload, gdriveSlice...)
	}
	if len(pixivFanboxDl.CreatorIds) > 0 {
		fanboxIds := getCreatorsPosts(
			pixivFanboxDl.CreatorIds,
			pixivFanboxDl.CreatorPageNums,
			config,
			pixivFanboxDlOptions.SessionCookies,
		)
		urlsSlice, gdriveSlice := getPostDetails(
			fanboxIds,
			config,
			pixivFanboxDlOptions,
		)
		urlsToDownload = append(urlsToDownload, urlsSlice...)
		gdriveUrlsToDownload = append(gdriveUrlsToDownload, gdriveSlice...)
	}

	if len(urlsToDownload) > 0 {
		request.DownloadUrls(
			urlsToDownload,
			&request.DlOptions{
				MaxConcurrency: utils.PIXIV_MAX_CONCURRENT_DOWNLOADS,
				Headers:        GetPixivFanboxHeaders(),
				Cookies:        pixivFanboxDlOptions.SessionCookies,
				UseHttp3:       false,
			},
			config,
		)
	}
	if pixivFanboxDlOptions.GDriveClient != nil && len(gdriveUrlsToDownload) > 0 {
		pixivFanboxDlOptions.GDriveClient.DownloadGdriveUrls(gdriveUrlsToDownload, config)
	}
}
