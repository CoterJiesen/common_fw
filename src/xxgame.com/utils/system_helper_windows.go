// +build windows

/*
windows操作系统的实现
*/

package utils

func daemonize() int {
	return 0
}

func flock(fd int) error {
	return nil
}

//从路径中获取文件名
func base(path string) string {
	return baseimpl(path, "\\")
}

//从路径中获取目录名
func dir(path string) string {
	return dirimpl(path, "\\")
}

//获取路径分割符
func getPathDel() string {
	return "\\"
}
