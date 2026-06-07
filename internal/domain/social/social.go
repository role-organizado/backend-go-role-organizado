package social

import (
	"regexp"
	"strings"
	"time"
)

// Constantes de limite de itens.
const (
	MaxPlaylists  = 3
	MaxAlbumLinks = 5
)

// --- Enums ---

// DressCodeTipo representa o tipo de dress code de um evento.
type DressCodeTipo string

const (
	DressCodeCasual    DressCodeTipo = "CASUAL"
	DressCodeEsporte   DressCodeTipo = "ESPORTE"
	DressCodeSocial    DressCodeTipo = "SOCIAL"
	DressCodeFormal    DressCodeTipo = "FORMAL"
	DressCodeFantasia  DressCodeTipo = "FANTASIA"
	DressCodeTematico  DressCodeTipo = "TEMATICO"
)

// PlaylistProvider representa o provedor de streaming de uma playlist.
type PlaylistProvider string

const (
	PlaylistSpotify  PlaylistProvider = "SPOTIFY"
	PlaylistYouTube  PlaylistProvider = "YOUTUBE"
	PlaylistOther    PlaylistProvider = "OTHER"
)

// AlbumProvider representa o provedor de álbum de fotos.
type AlbumProvider string

const (
	AlbumGooglePhotos AlbumProvider = "GOOGLE_PHOTOS"
	AlbumICloud       AlbumProvider = "ICLOUD"
	AlbumOther        AlbumProvider = "OTHER"
)

// EventoFase representa a fase mínima exigida para operações do domínio social.
// Reflete o enum EventoFase do Java; usado na validação de pré-condição dos UCs.
type EventoFase string

const (
	FaseAguardandoAceite EventoFase = "AGUARDANDO_ACEITE"
)

// --- Sub-entidades ---

// DressCode representa a configuração de dress code de um evento.
type DressCode struct {
	Tipo               DressCodeTipo `bson:"tipo"`
	DescricaoTematico  string        `bson:"descricaoTematico,omitempty"`
}

// PlaylistLink representa um link de playlist associado a um evento.
type PlaylistLink struct {
	ID       string           `bson:"id"`
	URL      string           `bson:"url"`
	Nome     string           `bson:"nome"`
	Provider PlaylistProvider `bson:"provider"`
	EmbedURL string           `bson:"embedUrl,omitempty"`
}

// BringListItem representa um item da lista colaborativa "trazer para o evento".
type BringListItem struct {
	ID            string     `bson:"id"`
	Nome          string     `bson:"nome"`
	Quantidade    string     `bson:"quantidade,omitempty"`
	ClaimedBy     string     `bson:"claimedBy,omitempty"`
	ClaimedByNome string     `bson:"claimedByNome,omitempty"`
	ClaimedAt     *time.Time `bson:"claimedAt,omitempty"`
}

// Checkin representa o registro de presença de um participante num evento.
type Checkin struct {
	UsuarioID string    `bson:"usuarioId"`
	Nome      string    `bson:"nome"`
	Timestamp time.Time `bson:"timestamp"`
}

// AlbumLink representa um link de álbum de fotos associado a um evento.
type AlbumLink struct {
	ID            string        `bson:"id"`
	URL           string        `bson:"url"`
	Nome          string        `bson:"nome"`
	Provider      AlbumProvider `bson:"provider"`
	AdicionadoPor string        `bson:"adicionadoPor"`
}

// --- Entidade raiz ---

// EventoSocialFeatures agrupa todos os recursos sociais de um evento.
// Mapeado na collection `evento_social_features` (compartilhada com o Java).
type EventoSocialFeatures struct {
	ID                 string         `bson:"_id,omitempty"`
	EventoID           string         `bson:"eventoId"`
	DressCode          *DressCode     `bson:"dressCode,omitempty"`
	Playlists          []PlaylistLink `bson:"playlists,omitempty"`
	BringList          []BringListItem `bson:"bringList,omitempty"`
	CheckinHabilitado  bool           `bson:"checkinHabilitado"`
	Checkins           []Checkin      `bson:"checkins,omitempty"`
	AlbumLinks         []AlbumLink    `bson:"albumLinks,omitempty"`
	CriadoEm           time.Time      `bson:"criadoEm"`
	AtualizadoEm       time.Time      `bson:"atualizadoEm"`
}

// --- Expressões regulares para detecção de providers ---

var (
	reSpotify  = regexp.MustCompile(`open\.spotify\.com/playlist/([a-zA-Z0-9]+)`)
	reYouTube  = regexp.MustCompile(`(?:youtu\.be/|youtube\.com/(?:playlist\?list=|watch\?.*list=))([a-zA-Z0-9_-]+)`)
)

// DetectPlaylistProvider analisa a URL e retorna o provider detectado e o embedURL
// correspondente.
//
// Regras:
//   - Spotify: regex open.spotify.com/playlist/{id} → embed https://open.spotify.com/embed/playlist/{id}?utm_source=generator&theme=0
//   - YouTube: regex youtu.be/ ou youtube.com/playlist?list= ou youtube.com/watch?...list= → embed https://www.youtube.com/embed/videoseries?list={id}
//   - OTHER: embedURL = ""
func DetectPlaylistProvider(url string) (PlaylistProvider, string) {
	if m := reSpotify.FindStringSubmatch(url); len(m) == 2 {
		embedURL := "https://open.spotify.com/embed/playlist/" + m[1] + "?utm_source=generator&theme=0"
		return PlaylistSpotify, embedURL
	}
	if m := reYouTube.FindStringSubmatch(url); len(m) == 2 {
		embedURL := "https://www.youtube.com/embed/videoseries?list=" + m[1]
		return PlaylistYouTube, embedURL
	}
	return PlaylistOther, ""
}

// DetectAlbumProvider analisa a URL e retorna o provider detectado.
//
// Regras:
//   - GOOGLE_PHOTOS se URL contém `photos.google.com` ou `google.com/photos`
//   - ICLOUD      se URL contém `icloud.com/photos` ou `icloud.com/sharedalbum`
//   - OTHER       caso contrário
func DetectAlbumProvider(url string) AlbumProvider {
	switch {
	case strings.Contains(url, "photos.google.com") || strings.Contains(url, "google.com/photos"):
		return AlbumGooglePhotos
	case strings.Contains(url, "icloud.com/photos") || strings.Contains(url, "icloud.com/sharedalbum"):
		return AlbumICloud
	default:
		return AlbumOther
	}
}
