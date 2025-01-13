package models

type ApplePlaylistTracksResponse struct {
	Data      []Data    `json:"data,omitempty"`
	Resources Resources `json:"resources,omitempty"`
	Meta      Meta      `json:"meta,omitempty"`
}

type Data struct {
	ID   string `json:"id,omitempty"`
	Type string `json:"type,omitempty"`
	Href string `json:"href,omitempty"`
}

type Attributes struct {
	Name       string `json:"name,omitempty"`
	ArtistName string `json:"artistName,omitempty"`
}

type LibrarySong struct {
	ID         string     `json:"id,omitempty"`
	Type       string     `json:"type,omitempty"`
	Href       string     `json:"href,omitempty"`
	Attributes Attributes `json:"attributes,omitempty"`
}

type Resources struct {
	LibrarySongs map[string]LibrarySong `json:"library-songs,omitempty"`
}

type Meta struct {
	Total int `json:"total,omitempty"`
}
