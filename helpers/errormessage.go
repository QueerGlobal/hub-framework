package helpers

import "strings"

type ErrorMessage struct {
	ErrorCode string
	Message   string
}

func IsDuplicateKey(errMessage string) bool {

	//TODO: Check for db type here and do different checks accordingly
	if strings.Contains(errMessage, "Duplicate") {
		return true
	}
	return false

}
