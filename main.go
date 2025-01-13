package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/go-querystring/query"
	"github.com/haileyok/sapple/models"
	_ "github.com/joho/godotenv/autoload"
	"github.com/labstack/echo/v4"
	"github.com/urfave/cli/v2"
)

var (
	AppleStorefront = ""

	AppleBaseUrl                    = "https://amp-api.music.apple.com"
	AppleListPlaylistsEndpoint      = "/v1/me/library/playlists/p.PO4Yc28O6xre"
	AppleListPlaylistTracksEndpoint = "/v1/me/library/playlists/%s/tracks"
	AppleSongsEndpoint              = "/v1/me/library/songs"

	SpotifyApiBaseUrl             = "https://api.spotify.com/v1"
	SpotifySearchEndpoint         = "/search"
	SpotifyPlaylistTracksEndpoint = "/playlists/%s/tracks"
	SpotifyPlaylistsEndpoint      = "/users/%s/playlists"
	SpotifyMeEndpoint             = "/me"

	SpotifyAccountsBaseUrl     = "https://accounts.spotify.com/api"
	SpotifyAccessTokenEndpoint = "/token"
)

type Engine struct {
	h struct {
		c  *http.Client
		mu sync.Mutex
	}
	ctx    context.Context
	config *EngineConfig

	spotifyAuth *models.SpotifyGetTokenResponse
	spotifyMe   *models.SpotifyMeResponse
}

type EngineConfig struct {
	appleJwt            string
	appleToken          string
	spotifyClientId     string
	spotifyClientSecret string
}

func NewEngine(c *cli.Context) (*Engine, error) {
	e := &Engine{
		ctx: c.Context,
	}

	e.h.c = &http.Client{
		Timeout: 2 * time.Second,
	}

	config := &EngineConfig{
		appleJwt:            c.String("apple-jwt"),
		appleToken:          c.String("apple-token"),
		spotifyClientId:     c.String("spotify-client-id"),
		spotifyClientSecret: c.String("spotify-client-secret"),
	}

	e.config = config

	return e, nil
}

func main() {
	app := &cli.App{
		Name:  "sapple client",
		Usage: "apple music to spotify transfer",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "apple-jwt",
				EnvVars: []string{"APPLE_JWT"},
			},
			&cli.StringFlag{
				Name:    "apple-token",
				EnvVars: []string{"APPLE_TOKEN"},
			},
			&cli.StringFlag{
				Name:    "spotify-client-id",
				EnvVars: []string{"SPOTIFY_CLIENT_ID"},
			},
			&cli.StringFlag{
				Name:    "spotify-client-secret",
				EnvVars: []string{"SPOTIFY_CLIENT_SECRET"},
			},
			&cli.StringFlag{},
		},
		Commands: []*cli.Command{
			run,
		},
	}

	app.RunAndExitOnError()
}

var run = &cli.Command{
	Name: "run",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name: "playlist-id",
		},
		&cli.BoolFlag{
			Name: "library",
		},
		&cli.StringFlag{
			Name:     "name",
			Required: true,
		},
	},
	Action: func(c *cli.Context) error {
		name := c.String("name")
		if name == "" {
			return fmt.Errorf("no name provided")
		}

		pid := c.String("playlist-id")
		library := c.Bool("library")
		if pid == "" && !library {
			return fmt.Errorf("no playlist id provided")
		}

		e, err := NewEngine(c)
		if err != nil {
			return err
		}

		spotifyAuth, err := e.spotifyCreateAccessToken()
		if err != nil {
			return err
		}
		e.spotifyAuth = spotifyAuth

		spotifyMe, err := e.spotifyGetMe()
		if err != nil {
			return err
		}
		e.spotifyMe = spotifyMe

		items := []models.LibrarySong{}
		total := 1

		fmt.Println("fetching items...")

		for len(items) < total {
			fmt.Printf("\rfetched %d/%d items", len(items), total)

			var res *models.ApplePlaylistTracksResponse
			var err error

			if library {
				res, err = e.appleLibraryTracks(100, len(items))
			} else {
				res, err = e.applePlaylistTracks(pid, 100, len(items))
			}

			if err != nil {
				return err
			}

			total = res.Meta.Total

			for _, ls := range res.Resources.LibrarySongs {
				items = append(items, ls)
			}
		}

		fmt.Println("\nsearching for items...")

		errs := 0
		notFound := 0
		sitems := []string{}
		total = len(items)

		for i, item := range items {
			q := item.Attributes.Name + " " + item.Attributes.ArtistName
			res, err := e.spotifySearchForTrack(q)
			if err != nil {
				errs++
				fmt.Println("error finding track:", err)
				continue
			}

			if len(res.Tracks.Items) == 0 {
				notFound++
				fmt.Println("could not find track", q)
				continue
			}

			sitems = append(sitems, res.Tracks.Items[0].Uri)

			fmt.Printf("\rsearched for %d/%d. found: %d. not found: %d. errors: %d", i+1, total, (i+1)-errs-notFound, notFound, errs)
		}

		fmt.Println("\ncreating new playlist...")

		res, err := e.spotifyCreatePlaylist(name)
		if err != nil {
			return err
		}

		fmt.Println("new playlist created!", res.Uri)

		npid := res.Id
		chunks := chunk(sitems, 100)

		fmt.Println("adding items to new playlist...")

		for i, c := range chunks {
			fmt.Printf("\radding chunk %d/%d", i+1, len(chunks))

			_, err := e.spotifyAddItemsToPlaylist(npid, c)
			if err != nil {
				return err
			}
		}

		fmt.Println("\nadded items to playlist!")

		return nil
	},
}

func (e *Engine) makeAppleRequest(endpoint string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(e.ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json;charset=utf-8")
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Referer", "https://music.apple.com")
	req.Header.Add("Origin", "https://music.apple.com")
	req.Header.Add("Accept-Encoding", "gzip, deflate, br")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36")
	req.Header.Add("Authorization", "Bearer "+e.config.appleJwt)
	req.Header.Add("media-user-token", e.config.appleToken)

	return req, nil
}

func (e *Engine) applePlaylistTracks(playlistId string, limit, offset int) (*models.ApplePlaylistTracksResponse, error) {
	req, err := e.makeAppleRequest(AppleBaseUrl + fmt.Sprintf(AppleListPlaylistTracksEndpoint+"?include=tracks&format[resources]=map&limit=%d&offset=%d", playlistId, limit, offset))
	if err != nil {
		return nil, err
	}

	res, err := e.h.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("error fetching playlist: %w", err)
	}

	j, err := gzipToString(res.Body)
	if err != nil {
		return nil, err
	}

	var pres models.ApplePlaylistTracksResponse
	if err := json.Unmarshal(j, &pres); err != nil {
		return nil, err
	}

	return &pres, nil
}

func (e *Engine) appleLibraryTracks(limit, offset int) (*models.ApplePlaylistTracksResponse, error) {
	req, err := e.makeAppleRequest(AppleBaseUrl + AppleSongsEndpoint + fmt.Sprintf("?include=library-songs&format[resources]=map&limit=%d&offset=%d&sort=-dateAdded", limit, offset))
	if err != nil {
		return nil, err
	}

	res, err := e.h.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("error fetching library: %w", err)
	}

	j, err := gzipToString(res.Body)
	if err != nil {
		return nil, err
	}

	var sres models.ApplePlaylistTracksResponse
	if err := json.Unmarshal(j, &sres); err != nil {
		return nil, err
	}

	return &sres, nil
}

func (e *Engine) makeSpotifyRequest(method, endpoint string, body *[]byte) (*http.Request, error) {
	var buf bytes.Buffer
	if body != nil {
		buf = *bytes.NewBuffer(*body)
	}

	req, err := http.NewRequestWithContext(e.ctx, method, endpoint, &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+e.spotifyAuth.AccessToken)

	return req, nil
}

func (e *Engine) spotifyCreatePlaylist(name string) (*models.SpotifyCreatePlaylistResponse, error) {
	params := &models.SpotifyCreatePlaylistRequest{
		Name:        name,
		Description: "",
		Public:      false,
	}

	j, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	req, err := e.makeSpotifyRequest("POST", fmt.Sprintf(SpotifyApiBaseUrl+SpotifyPlaylistsEndpoint, e.spotifyMe.Id), &j)
	if err != nil {
		return nil, err
	}

	res, err := e.h.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var cres models.SpotifyCreatePlaylistResponse
	if err := json.NewDecoder(res.Body).Decode(&cres); err != nil {
		return nil, err
	}

	return &cres, nil
}

func (e *Engine) spotifyAddItemsToPlaylist(pid string, items []string) (*models.SpotifyAddItemsResponse, error) {
	params := &models.SpotifyAddItemsRequest{
		Uris: items,
	}

	j, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	ustr := fmt.Sprintf(SpotifyApiBaseUrl+SpotifyPlaylistTracksEndpoint, pid)
	req, err := e.makeSpotifyRequest("POST", ustr, &j)
	if err != nil {
		return nil, err
	}

	res, err := e.h.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var ares models.SpotifyAddItemsResponse
	if err := json.NewDecoder(res.Body).Decode(&ares); err != nil {
		return nil, err
	}

	return &ares, nil
}

func (e *Engine) spotifySearchForTrack(q string) (*models.SpotifySearchResponse, error) {
	q = strings.ReplaceAll(q, " ", "+")
	params := &models.SpotifySearchRequest{
		Q:      q,
		Type:   "track",
		Market: "US",
		Limit:  1,
		Offset: 0,
	}

	vals, err := query.Values(params)
	if err != nil {
		return nil, err
	}

	ustr := fmt.Sprintf("%s%s?%s", SpotifyApiBaseUrl, SpotifySearchEndpoint, vals.Encode())
	req, err := e.makeSpotifyRequest("GET", ustr, nil)
	if err != nil {
		return nil, err
	}

	res, err := e.h.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var sres models.SpotifySearchResponse
	if err := json.NewDecoder(res.Body).Decode(&sres); err != nil {
		return nil, err
	}

	return &sres, err
}

func (e *Engine) spotifyGetMe() (*models.SpotifyMeResponse, error) {
	req, err := e.makeSpotifyRequest("GET", SpotifyApiBaseUrl+SpotifyMeEndpoint, nil)
	if err != nil {
		return nil, err
	}

	res, err := e.h.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var mres models.SpotifyMeResponse
	if err := json.NewDecoder(res.Body).Decode(&mres); err != nil {
		return nil, err
	}

	return &mres, nil
}

func (e *Engine) spotifyCreateAccessToken() (*models.SpotifyGetTokenResponse, error) {
	ech := echo.New()
	srv := http.Server{
		Addr:    ":8888",
		Handler: ech,
	}

	var wg sync.WaitGroup

	var code string
	handle := func(c echo.Context) error {
		code = c.QueryParam("code")
		wg.Done()
		return nil
	}

	ech.Add("GET", "/callback", handle)

	fmt.Printf("go to the url and sign in: https://accounts.spotify.com/authorize?response_type=code&client_id=%s&scope=%s&redirect_uri=%s&state=123456\n", e.config.spotifyClientId, url.QueryEscape("user-library-modify playlist-read-private playlist-modify-private playlist-modify-public"), url.QueryEscape("http://localhost:8888/callback"))

	wg.Add(1)
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			fmt.Println("error starting server", err)
		}
	}()

	wg.Wait()

	params := &models.SpotifyGetTokenRequest{
		Code:        code,
		RedirectUri: "http://localhost:8888/callback",
		GrantType:   "authorization_code",
	}

	vals, err := query.Values(params)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(e.ctx, "POST", SpotifyAccountsBaseUrl+SpotifyAccessTokenEndpoint, strings.NewReader(vals.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	req.Header.Add("authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(e.config.spotifyClientId+":"+e.config.spotifyClientSecret))))

	res, err := e.h.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var ares models.SpotifyGetTokenResponse
	if err := json.NewDecoder(res.Body).Decode(&ares); err != nil {
		return nil, err
	}

	return &ares, err
}

func gzipToString(r io.Reader) ([]byte, error) {
	gzreader, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	return io.ReadAll(gzreader)
}

func chunk[T any](array []T, size int) [][]T {
	if size <= 0 {
		return nil
	}

	var chunks [][]T
	for i := 0; i < len(array); i += size {
		end := i + size
		if end > len(array) {
			end = len(array)
		}
		chunks = append(chunks, array[i:end])
	}

	return chunks
}
