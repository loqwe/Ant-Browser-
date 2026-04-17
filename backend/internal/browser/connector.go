package browser

import "strings"

var defaultVerificationURLs = []string{
	"https://ippure.com/",
	"https://iplark.com/",
	"https://ping0.cc/",
}

// BuildLaunchArgs 构建启动参数
func BuildLaunchArgs(args []string, profile *Profile) []string {
	args = append(args, defaultVerificationURLs...)
	return args
}

// BuildExtensionLoadArgs 构建扩展加载参数
func BuildExtensionLoadArgs(args []string, paths []string) []string {
	if len(paths) == 0 {
		return args
	}
	joined := strings.Join(paths, ",")
	args = append(args, "--disable-extensions-except="+joined)
	args = append(args, "--load-extension="+joined)
	return args
}
