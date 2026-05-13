package buildinfo

import "fmt"

var (
	commitDateTime = "unknown"
	commitID       = "unknown"
	branchName     = "unknown"
)

func FooterLabel() string {
	return fmt.Sprintf("Code by Yuhao@jiansutech.com - %s - %s - %s", commitDateTime, commitID, branchName)
}
