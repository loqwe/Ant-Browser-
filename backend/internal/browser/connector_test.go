package browser

import (
	"reflect"
	"testing"
)

func TestBuildLaunchArgsAppendsDefaultVerificationURLs(t *testing.T) {
	t.Parallel()

	baseArgs := []string{"--disable-sync"}
	got := BuildLaunchArgs(append([]string{}, baseArgs...), &Profile{})
	want := []string{
		"--disable-sync",
		"https://ippure.com/",
		"https://iplark.com/",
		"https://ping0.cc/",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildLaunchArgs 结果错误:\n got=%v\nwant=%v", got, want)
	}
}

func TestBuildExtensionLoadArgsAppendsManagedExtensionFlags(t *testing.T) {
	t.Parallel()
	baseArgs := []string{"--disable-sync"}
	paths := []string{`D:\ext\a`, `D:\ext\b`}
	got := BuildExtensionLoadArgs(append([]string{}, baseArgs...), paths)
	want := []string{
		"--disable-sync",
		"--disable-extensions-except=D:\\ext\\a,D:\\ext\\b",
		"--load-extension=D:\\ext\\a,D:\\ext\\b",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildExtensionLoadArgs 结果错误:\n got=%v\nwant=%v", got, want)
	}
}
