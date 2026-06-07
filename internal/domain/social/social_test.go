package social_test

import (
	"testing"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
	"github.com/stretchr/testify/assert"
)

func TestDetectPlaylistProvider(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		wantProvider    social.PlaylistProvider
		wantEmbedURLSub string // substring that must be present in embedURL (empty = embedURL must be "")
	}{
		{
			name:            "Spotify playlist URL",
			url:             "https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M",
			wantProvider:    social.PlaylistSpotify,
			wantEmbedURLSub: "https://open.spotify.com/embed/playlist/37i9dQZF1DXcBWIGoYBM5M",
		},
		{
			name:            "Spotify playlist URL with query params",
			url:             "https://open.spotify.com/playlist/5FjDHOF4iWGHnBPrhem9bq?si=abc123",
			wantProvider:    social.PlaylistSpotify,
			wantEmbedURLSub: "https://open.spotify.com/embed/playlist/5FjDHOF4iWGHnBPrhem9bq",
		},
		{
			name:            "Spotify embed URL contains generator theme param",
			url:             "https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M",
			wantProvider:    social.PlaylistSpotify,
			wantEmbedURLSub: "utm_source=generator&theme=0",
		},
		{
			name:            "YouTube playlist via youtube.com/playlist?list=",
			url:             "https://www.youtube.com/playlist?list=PLxyz123ABC",
			wantProvider:    social.PlaylistYouTube,
			wantEmbedURLSub: "https://www.youtube.com/embed/videoseries?list=PLxyz123ABC",
		},
		{
			name:            "YouTube playlist via youtube.com/watch?v=...&list=",
			url:             "https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLxyz123ABC",
			wantProvider:    social.PlaylistYouTube,
			wantEmbedURLSub: "https://www.youtube.com/embed/videoseries?list=PLxyz123ABC",
		},
		{
			name:            "YouTube short link youtu.be/",
			url:             "https://youtu.be/PLxyz123ABC",
			wantProvider:    social.PlaylistYouTube,
			wantEmbedURLSub: "https://www.youtube.com/embed/videoseries?list=PLxyz123ABC",
		},
		{
			name:            "Unknown URL returns OTHER with empty embedURL",
			url:             "https://music.apple.com/br/playlist/top-100-brasil/pl.d25f5d1181894928af76c85c967f8f31",
			wantProvider:    social.PlaylistOther,
			wantEmbedURLSub: "",
		},
		{
			name:            "Empty URL returns OTHER",
			url:             "",
			wantProvider:    social.PlaylistOther,
			wantEmbedURLSub: "",
		},
		{
			name:            "SoundCloud URL returns OTHER",
			url:             "https://soundcloud.com/user/sets/my-playlist",
			wantProvider:    social.PlaylistOther,
			wantEmbedURLSub: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, embedURL := social.DetectPlaylistProvider(tt.url)
			assert.Equal(t, tt.wantProvider, provider)
			if tt.wantEmbedURLSub == "" {
				assert.Empty(t, embedURL)
			} else {
				assert.Contains(t, embedURL, tt.wantEmbedURLSub)
			}
		})
	}
}

func TestDetectAlbumProvider(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		wantProvider social.AlbumProvider
	}{
		{
			name:         "Google Photos direct URL",
			url:          "https://photos.google.com/share/abc123",
			wantProvider: social.AlbumGooglePhotos,
		},
		{
			name:         "Google Photos via google.com/photos path",
			url:          "https://www.google.com/photos/photo/abc123",
			wantProvider: social.AlbumGooglePhotos,
		},
		{
			name:         "iCloud Photos URL",
			url:          "https://www.icloud.com/photos/abc123",
			wantProvider: social.AlbumICloud,
		},
		{
			name:         "iCloud Shared Album URL",
			url:          "https://www.icloud.com/sharedalbum/#B1Uxyz",
			wantProvider: social.AlbumICloud,
		},
		{
			name:         "Unknown URL returns OTHER",
			url:          "https://flickr.com/photos/user/sets/72157720000000",
			wantProvider: social.AlbumOther,
		},
		{
			name:         "Empty URL returns OTHER",
			url:          "",
			wantProvider: social.AlbumOther,
		},
		{
			name:         "Amazon Photos returns OTHER",
			url:          "https://www.amazon.com/photos/share/abc123",
			wantProvider: social.AlbumOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := social.DetectAlbumProvider(tt.url)
			assert.Equal(t, tt.wantProvider, provider)
		})
	}
}
