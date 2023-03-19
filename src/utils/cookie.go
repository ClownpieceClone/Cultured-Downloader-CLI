package utils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
)

// Returns the cookie info for the specified site
//
// Will panic if the site does not match any of the cases
func GetSessionCookieInfo(site string) *cookieInfo {
	switch site {
	case FANTIA:
		return &cookieInfo{
			Domain:   "fantia.jp",
			Name:     "_session_id",
			SameSite: http.SameSiteLaxMode,
		}
	case PIXIV_FANBOX:
		return &cookieInfo{
			Domain:   ".fanbox.cc",
			Name:     "FANBOXSESSID",
			SameSite: http.SameSiteNoneMode,
		}
	case PIXIV:
		return &cookieInfo{
			Domain:   ".pixiv.net",
			Name:     "PHPSESSID",
			SameSite: http.SameSiteNoneMode,
		}
	case KEMONO:
		return &cookieInfo{
			Domain:   "kemono.party",
			Name:     "session",
			SameSite: http.SameSiteNoneMode,
		}
	default:
		panic(
			fmt.Errorf(
				"error %d, invalid site, %q in GetSessionCookieInfo",
				DEV_ERROR,
				site,
			),
		)
	}
}

// For the exported cookies in JSON instead of Netscape format
type ExportedCookies []struct {
	Domain   string  `json:"domain"`
	Expire   float64 `json:"expirationDate"`
	HttpOnly bool    `json:"httpOnly"`
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	Secure   bool    `json:"secure"`
	Value    string  `json:"value"`
	Session  bool    `json:"session"`
}

type cookieInfoArgs struct {
	name     string
	sameSite http.SameSite
}

func parseTxtCookieFile(f *os.File, filePath string, cookieArgs *cookieInfoArgs) ([]*http.Cookie, error) {
	var cookies []*http.Cookie
	reader := bufio.NewReader(f)
	for {
		lineBytes, err := ReadLine(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf(
				"error %d: reading cookie file at %s, more info => %v",
				OS_ERROR,
				filePath,
				err,
			)
		}

		line := strings.TrimSpace(string(lineBytes))
		if line == "" || strings.HasPrefix(line, "#") {
			continue // skip empty lines and comments
		}

		// split the line
		cookieInfos := strings.Split(line, "\t")
		if len(cookieInfos) < 7 {
			continue // too few values will be ignored
		}

		cookieName := cookieInfos[5]
		if cookieName != cookieArgs.name {
			continue // not the session cookie
		}

		// parse the values
		cookie := http.Cookie{
			Name:     cookieName,
			Value:    cookieInfos[6],
			Domain:   cookieInfos[0],
			Path:     cookieInfos[2],
			Secure:   cookieInfos[3] == "TRUE",
			HttpOnly: true,
			SameSite: cookieArgs.sameSite,
		}

		expiresUnixStr := cookieInfos[4]
		if expiresUnixStr != "" {
			expiresUnixInt, err := strconv.Atoi(expiresUnixStr)
			if err != nil {
				// should never happen but just in case
				errMsg := fmt.Sprintf(
					"error %d: parsing cookie expiration time, %q, more info => %v",
					UNEXPECTED_ERROR,
					expiresUnixStr,
					err,
				)
				color.Red(errMsg)
				continue
			}
			if expiresUnixInt > 0 {
				cookie.Expires = time.Unix(int64(expiresUnixInt), 0)
			}
		}
		cookies = append(cookies, &cookie)
	}
	return cookies, nil
}

func parseJsonCookieFile(f *os.File, filePath string, cookieArgs *cookieInfoArgs) ([]*http.Cookie, error) {
	var cookies []*http.Cookie
	var exportedCookies ExportedCookies
	if err := json.NewDecoder(f).Decode(&exportedCookies); err != nil {
		return nil, fmt.Errorf(
			"error %d: failed to decode cookie JSON file at %s, more info => %v",
			JSON_ERROR,
			filePath,
			err,
		)
	}

	for _, cookie := range exportedCookies {
		if cookie.Name != cookieArgs.name {
			// not the session cookie
			continue
		}

		parsedCookie := &http.Cookie{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Domain:   cookie.Domain,
			Path:     cookie.Path,
			Secure:   cookie.Secure,
			HttpOnly: cookie.HttpOnly,
			SameSite: cookieArgs.sameSite,
		}
		if !cookie.Session {
			parsedCookie.Expires = time.Unix(int64(cookie.Expire), 0)
		}

		cookies = append(cookies, parsedCookie)
	}
	return cookies, nil
}

// parse the Netscape cookie file generated by extensions like Get cookies.txt LOCALLY
func ParseNetscapeCookieFile(filePath, sessionId, website string) ([]*http.Cookie, error) {
	if filePath != "" && sessionId != "" {
		return nil, fmt.Errorf(
			"error %d: cannot use both cookie file and session id flags",
			INPUT_ERROR,
		)
	}

	sessionCookieInfo := GetSessionCookieInfo(website)
	sessionCookieName := sessionCookieInfo.Name
	sessionCookieSameSite := sessionCookieInfo.SameSite

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf(
			"error %d: opening cookie file at %s, more info => %v",
			OS_ERROR,
			filePath,
			err,
		)
	}
	defer f.Close()

	cookieArgs := &cookieInfoArgs{
		name:     sessionCookieName,
		sameSite: sessionCookieSameSite,
	}
	var cookies []*http.Cookie
	if ext := filepath.Ext(filePath); ext == ".txt" {
		cookies, err = parseTxtCookieFile(f, filePath, cookieArgs)
	} else if ext == ".json" {
		cookies, err = parseJsonCookieFile(f, filePath, cookieArgs)
	} else {
		err = fmt.Errorf(
			"error %d: invalid cookie file extension, %q, at %s...\nOnly .txt and .json files are supported",
			INPUT_ERROR,
			ext,
			filePath,
		)
	}

	if err != nil {
		return nil, err
	}

	if len(cookies) == 0 {
		return nil, fmt.Errorf(
			"error %d: no session cookie found in cookie file at %s for website %s",
			INPUT_ERROR,
			filePath,
			GetReadableSiteStr(website),
		)
	}
	return cookies, nil
}
