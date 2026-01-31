package app

import (
	conf "mistletoe/internal/config"
)

import (
	"testing"
)

func TestIsPrFromConfiguredRepo(t *testing.T) {
	urlStr := "https://github.com/owner/repo.git"
	urlStrNoGit := "https://github.com/owner/repo"
	urlStrOther := "https://github.com/owner/other"

	tests := []struct {
		name               string
		prHeadURL          *string
		configCanonicalURL string
		want               bool
	}{
		{
			name:               "Exact match",
			prHeadURL:          &urlStr,
			configCanonicalURL: "https://github.com/owner/repo.git",
			want:               true,
		},
		{
			name:               "Match with missing .git in PR",
			prHeadURL:          &urlStrNoGit,
			configCanonicalURL: "https://github.com/owner/repo.git",
			want:               true,
		},
		{
			name:               "Match with missing .git in conf.Config",
			prHeadURL:          &urlStr,
			configCanonicalURL: "https://github.com/owner/repo",
			want:               true,
		},
		{
			name:               "Case insensitive match",
			prHeadURL:          strPtr("https://GitHub.com/Owner/Repo.git"),
			configCanonicalURL: "https://github.com/owner/repo.git",
			want:               true,
		},
		{
			name:               "No match",
			prHeadURL:          &urlStrOther,
			configCanonicalURL: "https://github.com/owner/repo.git",
			want:               false,
		},
		{
			name:               "Nil PR URL (Fallback)",
			prHeadURL:          nil,
			configCanonicalURL: "https://github.com/owner/repo.git",
			want:               true, // Safe default as per implementation
		},
		{
			name:               "Empty PR URL (Fallback)",
			prHeadURL:          strPtr(""),
			configCanonicalURL: "https://github.com/owner/repo.git",
			want:               true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := PrInfo{
				HeadRepository: conf.Repository{
					URL: tt.prHeadURL,
				},
			}
			if got := isPrFromConfiguredRepo(pr, tt.configCanonicalURL); got != tt.want {
				t.Errorf("isPrFromConfiguredRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatePrPermissionAndOverwrite(t *testing.T) {
	tests := []struct {
		name        string
		item        PrInfo
		currentUser string
		overwrite   bool
		wantError   bool
	}{
		{
			name:        "Success - Same user, editable",
			item:        PrInfo{ViewerCanEditFiles: true, Author: Author{Login: "me"}},
			currentUser: "me",
			overwrite:   false,
			wantError:   false,
		},
		{
			name:        "Fail - Not editable",
			item:        PrInfo{ViewerCanEditFiles: false, Author: Author{Login: "me"}},
			currentUser: "me",
			overwrite:   false,
			wantError:   true,
		},
		{
			name:        "Fail - Different user, no overwrite",
			item:        PrInfo{ViewerCanEditFiles: true, Author: Author{Login: "other"}},
			currentUser: "me",
			overwrite:   false,
			wantError:   true,
		},
		{
			name:        "Success - Different user, overwrite",
			item:        PrInfo{ViewerCanEditFiles: true, Author: Author{Login: "other"}},
			currentUser: "me",
			overwrite:   true,
			wantError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePrPermissionAndOverwrite("repo", tt.item, tt.currentUser, tt.overwrite)
			if (err != nil) != tt.wantError {
				t.Errorf("got error %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
