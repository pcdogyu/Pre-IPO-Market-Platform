package buildinfo

import "testing"

func TestFooterLabelUsesBuildMetadata(t *testing.T) {
	originalCommitDateTime := commitDateTime
	originalCommitID := commitID
	originalBranchName := branchName
	t.Cleanup(func() {
		commitDateTime = originalCommitDateTime
		commitID = originalCommitID
		branchName = originalBranchName
	})

	commitDateTime = "2026-05-13 16:20"
	commitID = "1234abcd"
	branchName = "main"

	got := FooterLabel()
	want := "Code by Yuhao@jiansutech.com - 2026-05-13 16:20 - 1234abcd - main"
	if got != want {
		t.Fatalf("FooterLabel() = %q, want %q", got, want)
	}
}
