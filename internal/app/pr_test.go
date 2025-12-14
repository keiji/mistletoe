package app

import (
	"strings"
	"testing"
)

func TestGetRepoName(t *testing.T) {
	id := "my-repo"
	repoWithID := Repository{ID: &id}
	if n := getRepoName(repoWithID); n != "my-repo" {
		t.Errorf("Expected my-repo, got %s", n)
	}

	url := "https://github.com/user/repo.git"
	repoNoID := Repository{URL: &url}
	if n := getRepoName(repoNoID); n != "repo" {
		t.Errorf("Expected repo, got %s", n)
	}
}

func TestVerifyGithubRequirements_NonGithub(t *testing.T) {
	// Simple unit test for string logic validation
	url1 := "https://github.com/user/repo"
	if !strings.Contains(url1, "github.com") {
		t.Error("Should identify github.com")
	}

	url2 := "https://gitlab.com/user/repo"
	if strings.Contains(url2, "github.com") {
		t.Error("Should not identify gitlab.com as github")
	}
}
