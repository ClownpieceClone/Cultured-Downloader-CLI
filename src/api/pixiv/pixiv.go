package pixiv

import (
	"fmt"

	"github.com/KJHJason/Cultured-Downloader-CLI/api/pixiv/models"
	"github.com/KJHJason/Cultured-Downloader-CLI/configs"
	"github.com/KJHJason/Cultured-Downloader-CLI/request"
	"github.com/KJHJason/Cultured-Downloader-CLI/spinner"
	"github.com/KJHJason/Cultured-Downloader-CLI/utils"
)

// Start the download process for Pixiv
func PixivDownloadProcess(config *configs.Config, pixivDl *PixivDl, pixivDlOptions *PixivDlOptions, pixivUgoiraOptions *UgoiraOptions) {
	var ugoiraToDownload []*models.Ugoira
	var artworksToDownload []map[string]string
	if len(pixivDl.ArtworkIds) > 0 {
		var artworksSlice []map[string]string
		var ugoiraSlice []*models.Ugoira
		if pixivDlOptions.MobileClient == nil {
			artworksSlice, ugoiraSlice = getMultipleArtworkDetails(
				pixivDl.ArtworkIds,
				utils.DOWNLOAD_PATH,
				config,
				pixivDlOptions.SessionCookies,
			)
		} else {
			artworksSlice, ugoiraSlice = pixivDlOptions.MobileClient.getMultipleArtworkDetails(
				pixivDl.ArtworkIds,
				utils.DOWNLOAD_PATH,
			)
		}
		artworksToDownload = append(artworksToDownload, artworksSlice...)
		ugoiraToDownload = append(ugoiraToDownload, ugoiraSlice...)
	}

	if len(pixivDl.IllustratorIds) > 0 {
		var artworksSlice []map[string]string
		var ugoiraSlice []*models.Ugoira
		if pixivDlOptions.MobileClient == nil {
			artworksSlice, ugoiraSlice = getMultipleIllustratorPosts(
				pixivDl.IllustratorIds,
				pixivDl.IllustratorPageNums,
				utils.DOWNLOAD_PATH,
				config,
				pixivDlOptions,
			)
		} else {
			artworksSlice, ugoiraSlice = pixivDlOptions.MobileClient.getMultipleIllustratorPosts(
				pixivDl.IllustratorIds,
				pixivDl.IllustratorPageNums,
				utils.DOWNLOAD_PATH,
				pixivDlOptions.ArtworkType,
			)
		}
		artworksToDownload = append(artworksToDownload, artworksSlice...)
		ugoiraToDownload = append(ugoiraToDownload, ugoiraSlice...)
	}

	if len(pixivDl.TagNames) > 0 {
		// loop through each tag and page number
		baseMsg := "Searching for artworks based on tag names on Pixiv [%d/" + fmt.Sprintf("%d]...", len(pixivDl.TagNames))
		progress := spinner.New(
			"pong",
			"fgHiYellow",
			fmt.Sprintf(
				baseMsg,
				0,
			),
			fmt.Sprintf(
				"Finished searching for artworks based on %d tag names on Pixiv!",
				len(pixivDl.TagNames),
			),
			fmt.Sprintf(
				"Finished with some errors while searching for artworks based on %d tag names on Pixiv!\nPlease refer to the logs for more details...",
				len(pixivDl.TagNames),
			),
			len(pixivDl.TagNames),
		)
		progress.Start()
		hasErr := false
		for idx, tagName := range pixivDl.TagNames {
			var artworksSlice []map[string]string
			var ugoiraSlice []*models.Ugoira
			if pixivDlOptions.MobileClient == nil {
				artworksSlice, ugoiraSlice, hasErr = tagSearch(
					tagName,
					utils.DOWNLOAD_PATH,
					pixivDl.TagNamesPageNums[idx],
					config,
					pixivDlOptions,
				)
			} else {
				artworksSlice, ugoiraSlice, hasErr = pixivDlOptions.MobileClient.tagSearch(
					tagName,
					utils.DOWNLOAD_PATH,
					pixivDl.TagNamesPageNums[idx],
					pixivDlOptions,
				)
			}
			artworksToDownload = append(artworksToDownload, artworksSlice...)
			ugoiraToDownload = append(ugoiraToDownload, ugoiraSlice...)
			progress.MsgIncrement(baseMsg)
		}
		progress.Stop(hasErr)
	}

	if len(artworksToDownload) > 0 {
		headers := GetPixivRequestHeaders()
		request.DownloadUrls(
			artworksToDownload,
			&request.DlOptions{
				MaxConcurrency: utils.PIXIV_MAX_CONCURRENT_DOWNLOADS,
				Headers:        headers,
				Cookies:        pixivDlOptions.SessionCookies,
				UseHttp3:       false,
			},
			config,
		)
	}
	if len(ugoiraToDownload) > 0 {
		downloadMultipleUgoira(
			ugoiraToDownload,
			config,
			pixivUgoiraOptions,
			pixivDlOptions.SessionCookies,
		)
	}
}
