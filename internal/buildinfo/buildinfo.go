package buildinfo

import "fmt"

var (
	commitDateTime = "未知"
	commitID       = "未知"
	branchName     = "未知"
)

func FooterLabel() string {
	return fmt.Sprintf("Code by Yuhao@jiansutech.com - %s - %s - %s", commitDateTime, commitID, branchName)
}
