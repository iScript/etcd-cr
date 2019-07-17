package flags

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/coreos/pkg/capnslog"
)

//
var plog = capnslog.NewPackageLogger("github.com/iScript/etcd-cr", "pkg/flags")

func SetFlagsFromEnv(prefix string, fs *flag.FlagSet) error {

	var err error
	alreadySet := make(map[string]bool)

	// visit 遍历已经设置的flag并且命令行有传
	fs.Visit(func(f *flag.Flag) {
		alreadySet[FlagToEnv(prefix, f.Name)] = true
		// map[ETCD_NAME:true]
	})

	usedEnvKey := make(map[string]bool) //map为引用类型，没从命令行传，但使用了环境变量的值有哪些

	// visitAll 遍历所有定义的flag
	fs.VisitAll(func(f *flag.Flag) {
		if serr := setFlagFromEnv(fs, prefix, f.Name, usedEnvKey, alreadySet, true); serr != nil {
			err = serr
		}
	})
	verifyEnv(prefix, usedEnvKey, alreadySet)
	return err
}

// flag name转为env字符串， 如version => ETCD_VERSION
func FlagToEnv(prefix, name string) string {
	return prefix + "_" + strings.ToUpper(strings.Replace(name, "-", "_", -1))
}

// 验证
func verifyEnv(prefix string, usedEnvKey, alreadySet map[string]bool) {

	// 遍历所有环境变量
	for _, env := range os.Environ() {

		kv := strings.SplitN(env, "=", 2)
		if len(kv) != 2 {
			plog.Warningf("found invalid env %s", env)
		}

		if usedEnvKey[kv[0]] {
			continue
		}
		//环境变量设置、命令行也传了。 冲突
		if alreadySet[kv[0]] {
			plog.Fatalf("conflicting environment variable %q is shadowed by corresponding command-line flag (either unset environment variable or disable flag)", kv[0])
		}
		// 不能识别
		if strings.HasPrefix(env, prefix+"_") {
			plog.Warningf("unrecognized environment variable %s", env)
		}
	}
}

type flagSetter interface {
	Set(fk string, fv string) error
}

func setFlagFromEnv(fs flagSetter, prefix, fname string, usedEnvKey, alreadySet map[string]bool, log bool) error {
	key := FlagToEnv(prefix, fname) //flag转为env的字符串 ETCD_NAME ETCD_VERSION 等

	// 如果已经定义了flag，但命令行没加
	if !alreadySet[key] {
		val := os.Getenv(key) // 获取环境变量定义的值 ， 如linux  export ETCD_NAME=test
		if val != "" {
			usedEnvKey[key] = true
			//从环境变量设置值
			if serr := fs.Set(fname, val); serr != nil {
				return fmt.Errorf("invalid value %q for %s: %v", val, key, serr)
			}
			if log {
				plog.Infof("recognized and used environment variable %s=%s", key, val)
			}
		}
	}
	return nil
}
