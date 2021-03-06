/*
提供系统相关的帮助函数
*/

package utils

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"go/build"
	"os"
	"path"
	"runtime"
	"strings"
	"time"
	"xxgame.com/component/log"
)

//获取进程名和运行路径
//返回值1:进程名
//返回值2:运行路径
func GetProcessNameAndPath() (string, string) {
	return Base(os.Args[0]), Dir(os.Args[0])
}

//获取当前进程pid
func GetPid() int {
	return os.Getpid()
}

//获取当前进程对应程序的版本信息
func ProcessVersion() string {
	return fmt.Sprintf("build os=%s arch=%s compiler=%s go_version=%s tags=%s",
		build.Default.GOOS,
		build.Default.GOARCH,
		build.Default.Compiler,
		runtime.Version(),
		build.Default.BuildTags)
}

//关闭终端输出(标准输入输出和错误输出)
func CloseTermOutput() {
	os.Stdin.Close()
	os.Stdout.Close()
	os.Stderr.Close()
}

//daemon化
func Daemonize() int {
	return daemonize()
}

//文件锁
func Flock(fd int) error {
	return flock(fd)
}

//从路径中获取文件名
func Base(path string) string {
	return base(path)
}

//从路径中获取目录名
func Dir(path string) string {
	return dir(path)
}

//从路径中获取文件名
//参数path:路径
//参数del:路径分隔符(不同操作系统不一样)
func baseimpl(path string, del string) string {
	if path == "" {
		return "."
	}
	// Strip trailing slashes.
	for len(path) > 0 && path[len(path)-1] == del[0] {
		path = path[0 : len(path)-1]
	}
	// Find the last element
	if i := strings.LastIndex(path, del); i >= 0 {
		path = path[i+1:]
	}
	// If empty now, it had only slashes.
	if path == "" {
		return del
	}
	return path
}

//从路径中获取目录名
//参数p:路径
//参数del:路径分隔符(不同操作系统不一样)
func dirimpl(p string, del string) string {
	dir, _ := path.Split(p)
	dir = path.Clean(dir)
	last := len(dir) - 1
	if last > 0 && dir[last] == del[0] {
		dir = dir[:last]
	}
	if dir == "" {
		dir = "."
	}
	return dir
}

//获取路径分割符
func GetPathDel() string {
	return getPathDel()
}

//封装一个函数入口，fun  为真正入口
func GoMain(fun func(), panicHandler func()) {
	defer func(pan bool) {
		if err := recover(); err != nil {
			if panicHandler != nil {
				panicHandler()
			}

			n, path := GetProcessNameAndPath()
			dumpName := fmt.Sprintf("dump_%s_%d_%d_%d:%d_%d_%d", n, time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Hour(), time.Now().Minute(), time.Now().Second())
			f, _ := os.Create(path + GetPathDel() + "../../log/" + n + GetPathDel() + dumpName + ".log")
			defer f.Close()
			buf := make([]byte, 1<<20)
			size := runtime.Stack(buf, false)

			f.WriteString(fmt.Sprintf("=== received SIGQUIT ===\n*** goroutine err...\n%s\n*** end\n", err))
			f.WriteString(fmt.Sprintf("=== received SIGQUIT ===\n*** goroutine dump...\n%s\n*** end\n", string(buf[:size])))
			f.Sync()
			fmt.Println(fmt.Sprintf("=== received SIGQUIT ===\n*** goroutine dump...\n%s\n*** end\n", string(buf[:size])))
			log.FlushAll()
			if pan {
				panic(" goroutine err")
			}
		}
	}(true)

	fun()
}

//判断uid是否属于机器人
func IsRobotUID(uid uint64) bool {
	return uid <= 1000000 || uid >= 8000000
}

func CheckDigest(digest, body, key []byte) bool {
	md5Body := make([]byte, 0)
	md5Body = append(md5Body, body...)
	md5Body = append(md5Body, key...)
	m5 := md5.Sum(md5Body)
	out := make([]byte, 0)
	for _, v := range m5 {
		out = append(out, v)
	}
	return bytes.Equal(digest, out)
}
