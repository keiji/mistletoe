package app

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
			name:               "Match with missing .git in Config",
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
				HeadRepository: Repository{
					URL: tt.prHeadURL,
				},
			}
			if got := isPrFromConfiguredRepo(pr, tt.configCanonicalURL); got != tt.want {
				t.Errorf("isPrFromConfiguredRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}
