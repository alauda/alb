package utils

import (
	"flag"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
)

func InitLog() {
	// 100 mb
	glog.MaxSize = 1024 * 1024 * 100
}

func RotateGlog(before time.Time) error {
	if f := flag.Lookup("log_dir"); f != nil {
		logDir := f.Value.String()
		files, err := ioutil.ReadDir(logDir)
		if err != nil {
			return err
		}
		skips := map[string]struct{}{}
		for _, f := range files {
			if dst, err := os.Readlink(f.Name()); err == nil {
				skips[f.Name()] = struct{}{}
				skips[dst] = struct{}{}
			}
		}
		for _, f := range files {
			if _, ok := skips[f.Name()]; ok {
				continue
			}
			// toy.zhuyans-MBP.halfcrazy.log.ERROR.20190318-202433.37428
			fields := strings.Split(f.Name(), ".")
			if len(fields) != 7 {
				continue
			}
			d, err := time.Parse("20060102", strings.SplitN(fields[len(fields)-2], "-", 2)[0])
			if err != nil {
				continue
			}
			if d.Before(before) {
				err := os.Remove(f.Name())
				if err != nil {
					return err
				}
				continue
			}
		}
	}
	return nil
}
