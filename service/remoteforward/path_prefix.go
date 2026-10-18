package remoteforward

import "strings"

// NormalizePathPrefix ensures a non-empty path prefix starts with / and ends with /.
func NormalizePathPrefix(pathPrefix string) string {
	if len(pathPrefix) == 0 {
		return pathPrefix
	}
	if !strings.HasPrefix(pathPrefix, "/") {
		pathPrefix = "/" + pathPrefix
	}
	if !strings.HasSuffix(pathPrefix, "/") {
		pathPrefix = pathPrefix + "/"
	}
	return pathPrefix
}
