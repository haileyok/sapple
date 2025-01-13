package models

type SpotifyGetTokenRequest struct {
	Code        string `url:"code"`
	RedirectUri string `url:"redirect_uri"`
	GrantType   string `url:"grant_type"`
}

type SpotifyGetTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

type SpotifyCreatePlaylistRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Public      bool   `json:"public"`
}

type SpotifyCreatePlaylistResponse struct {
	Id   string `json:"id"`
	Href string `json:"href"`
	Uri  string `json:"uri"`
}

type SpotifyAddItemsRequest struct {
	Position *int     `json:"position,omitempty"`
	Uris     []string `json:"uris"`
}

type SpotifyAddItemsResponse struct {
	SnapshotID string `json:"snapshot_id"`
}

type SpotifySearchRequest struct {
	Q      string `url:"q"`
	Type   string `url:"type"`
	Market string `url:"market"`
	Limit  int    `url:"limit"`
	Offset int    `url:"offset"`
}

type SpotifySearchResponse struct {
	Tracks SpotifySearchTracks `json:"tracks"`
}

type SpotifySearchTracks struct {
	Href  string               `json:"href"`
	Items []SpotifyTrackObject `json:"items"`
}

type SpotifyTrackObject struct {
	Id  string `json:"id"`
	Uri string `json:"uri"`
}

type SpotifyMeResponse struct {
	Id string `json:"id"`
}
