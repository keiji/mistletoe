package app

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestCalculateSnapshotIdentifier(t *testing.T) {
	// Logic:
	// sort by ID
	// for each: if branch != nil && branch != "" -> branch
	//           else if revision != nil -> revision
	// concat with ","
	// sha256

	r1ID := "repo1"
	r1Branch := "feature/abc"
	r1Rev := "rev1"
	r1 := Repository{
		ID:       &r1ID,
		Branch:   &r1Branch,
		Revision: &r1Rev,
	}

	r2ID := "repo2"
	r2Rev := "rev2"
	// Detached HEAD or no branch specified
	r2 := Repository{
		ID:       &r2ID,
		Branch:   nil,
		Revision: &r2Rev,
	}

	repos := []Repository{r2, r1} // Unsorted input

	// Expected string before hash:
	// repo1 -> feature/abc
	// repo2 -> rev2
	// "feature/abc,rev2"

	expectedStr := "feature/abc,rev2"
	hash := sha256.Sum256([]byte(expectedStr))
	expectedID := hex.EncodeToString(hash[:])

	id := CalculateSnapshotIdentifier(repos)

	if id != expectedID {
		t.Errorf("Expected ID %s, got %s", expectedID, id)
	}
}

func TestCalculateSnapshotIdentifier_BranchPriority(t *testing.T) {
	// Even if revision is present, branch should be used if available
	r1ID := "repo1"
	r1Branch := "main"
	r1Rev := "aabbcc"
	r1 := Repository{
		ID:       &r1ID,
		Branch:   &r1Branch,
		Revision: &r1Rev,
	}

	repos := []Repository{r1}

	expectedStr := "main"
	hash := sha256.Sum256([]byte(expectedStr))
	expectedID := hex.EncodeToString(hash[:])

	id := CalculateSnapshotIdentifier(repos)

	if id != expectedID {
		t.Errorf("Expected ID %s, got %s", expectedID, id)
	}
}

func TestCalculateSnapshotIdentifier_RevisionFallback(t *testing.T) {
	// If branch is empty string or nil, use revision
	r1ID := "repo1"
	r1Branch := ""
	r1Rev := "112233"
	r1 := Repository{
		ID:       &r1ID,
		Branch:   &r1Branch,
		Revision: &r1Rev,
	}

	repos := []Repository{r1}

	expectedStr := "112233"
	hash := sha256.Sum256([]byte(expectedStr))
	expectedID := hex.EncodeToString(hash[:])

	id := CalculateSnapshotIdentifier(repos)

	if id != expectedID {
		t.Errorf("Expected ID %s, got %s", expectedID, id)
	}
}
