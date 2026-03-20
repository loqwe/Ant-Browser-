package browser

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
