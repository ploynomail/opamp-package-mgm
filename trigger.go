package opamppackagemgm

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"go.uber.org/zap/zapcore"
)

const plat = runtime.GOOS + "-" + runtime.GOARCH // ex: linux-amd64
const upcktimePath = "cktime"

type TriggerUpdater interface {
	Trigger(ctx context.Context) chan UpdatePackageInfo
}

type RemoteFileCheckTrigger struct {
	Dir               string        // store the next update time
	ApiUrl            string        // the url to check for updates
	BinURL            string        // the url to download the binary
	CmdName           string        // the name of the command
	CheckTimeDuration time.Duration // how often to check for updates
	log               Loggerr
}

func NewRemoteFileCheckTrigger(
	url,
	binURL,
	cmdName string,
	dir string,
	checkTimeDuration time.Duration,
	log Loggerr,
) TriggerUpdater {
	return &RemoteFileCheckTrigger{
		ApiUrl:            url,
		BinURL:            binURL,
		CmdName:           cmdName,
		Dir:               dir,
		CheckTimeDuration: checkTimeDuration,
		log:               log,
	}
}

func (f *RemoteFileCheckTrigger) Trigger(ctx context.Context) chan UpdatePackageInfo {
	ch := make(chan UpdatePackageInfo)
	checkTick := time.NewTicker(f.CheckTimeDuration)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-checkTick.C:
				f.check(ch)
			}
		}
	}()
	return ch
}

func (f *RemoteFileCheckTrigger) check(ch chan UpdatePackageInfo) {
	// check for updates
	if f.NextUpdate().Before(time.Now()) {
		info, err := f.getInfo()
		if err != nil {
			f.log.Log(zapcore.ErrorLevel, err.Error())
			return
		}
		if info.Version != "" {
			ch <- UpdatePackageInfo{
				Version:     info.Version,
				ContentHash: info.Sha256,
				IsPatch:     info.IsPatch,
				DownloadUrl: fmt.Sprintf("%s/%s/%s/%s.gz", f.BinURL, f.CmdName, info.Version, plat),
				Signature:   nil,
			}
		}
		isComplate := f.SetUpdateTime()
		if !isComplate {
			f.log.Log(zapcore.ErrorLevel, "error setting next update time")
			return
		}
	} else {
		f.log.Log(zapcore.DebugLevel, "the next update checkpoint has not yet arrived")
	}
}

func (f *RemoteFileCheckTrigger) getInfo() (*Info, error) {
	url := f.ApiUrl + "/" + url.QueryEscape(f.CmdName) + "/" + url.QueryEscape(plat) + ".json"
	resp, err := http.Get(url)
	// get info
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bad http status from %s: %v", url, resp.Status)
	}
	var u Info
	err = json.NewDecoder(resp.Body).Decode(&u)
	if err != nil {
		return nil, err
	}
	if len(u.Sha256) != sha256.Size {
		return nil, errors.New("bad cmd hash in info")
	}
	return &u, nil
}

func (f *RemoteFileCheckTrigger) getExecRelativeDir(dir string) string {
	filename, _ := os.Executable()
	path := filepath.Join(filepath.Dir(filename), dir)
	return path
}

func (f *RemoteFileCheckTrigger) NextUpdate() time.Time {
	path := f.getExecRelativeDir(f.Dir + upcktimePath)
	nextTime := readTime(path)
	return nextTime
}

// SetUpdateTime writes the next update time to the state file
func (f *RemoteFileCheckTrigger) SetUpdateTime() bool {
	path := f.getExecRelativeDir(f.Dir + upcktimePath)
	wait := f.CheckTimeDuration
	// Add 1 to random time since max is not included
	waitrand := time.Duration(rand.Intn(3)) * time.Second

	return writeTime(path, time.Now().Add(wait+waitrand))
}

// json file example
// {
// 	"testagent": {
// 		"version": "v3",
// 		"download_url": "/pkg/testagent/v3/linux",
// 		"content_hash": "sMMSjZf4pamQEkjLWM65IAUYJ8BVi6ImOEwTVL33LxM="
// 	}
// }

type LocalFileCheckTrigger struct {
	Dir               string        // store the next update time
	CheckPath         string        // the path to check for updates
	BinURL            string        // the url to download the binary
	CmdName           string        // the name of the command
	CheckTimeDuration time.Duration // how often to check for updates
	log               Loggerr
}

func NewLocalFileCheckTrigger(
	checkPath,
	binURL,
	cmdName string,
	dir string,
	checkTimeDuration time.Duration,
	log Loggerr,
) TriggerUpdater {
	return &LocalFileCheckTrigger{
		CheckPath:         checkPath,
		BinURL:            binURL,
		CmdName:           cmdName,
		Dir:               dir,
		CheckTimeDuration: checkTimeDuration,
		log:               log,
	}
}

func (f *LocalFileCheckTrigger) Trigger(ctx context.Context) chan UpdatePackageInfo {
	ch := make(chan UpdatePackageInfo)
	checkTick := time.NewTicker(f.CheckTimeDuration)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-checkTick.C:
				f.check(ch)
			}
		}
	}()
	return ch
}

func (f *LocalFileCheckTrigger) check(ch chan UpdatePackageInfo) {
	// check for updates
	if f.NextUpdate().Before(time.Now()) {
		info, err := f.getInfo()
		if err != nil {
			f.log.Log(zapcore.ErrorLevel, "error getting info", zapcore.Field{Key: "error", Type: zapcore.ErrorType, Interface: err})
			return
		}
		if info == nil {
			return
		}
		if info.Version != "" {
			ch <- UpdatePackageInfo{
				Version:     info.Version,
				ContentHash: info.ContentHash,
				IsPatch:     info.IsPatch,
				DownloadUrl: fmt.Sprintf("%s%s", f.BinURL, info.DownloadUrl),
				Signature:   nil,
			}
		}
		isComplate := f.SetUpdateTime()
		if !isComplate {
			f.log.Log(zapcore.ErrorLevel, "error setting next update time")
			return
		}
	} else {
		f.log.Log(zapcore.DebugLevel, "the next update checkpoint has not yet arrived")
	}
}

func (f *LocalFileCheckTrigger) getInfo() (*UpdatePackageInfo, error) {
	path := f.CheckPath
	var infoFileContent []byte
	infoFileContent, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var u map[string]*UpdatePackageInfo
	err = json.Unmarshal(infoFileContent, &u)
	if err != nil {
		return nil, err
	}
	if u[f.CmdName] == nil {
		return nil, nil
	}
	if len(u[f.CmdName].ContentHash) != sha256.Size {
		return nil, errors.New("bad cmd hash in info")
	}
	return u[f.CmdName], nil
}

func (f *LocalFileCheckTrigger) getExecRelativeDir(dir string) string {
	filename, _ := os.Executable()
	path := filepath.Join(filepath.Dir(filename), dir)
	return path
}

func (f *LocalFileCheckTrigger) NextUpdate() time.Time {
	path := f.getExecRelativeDir(f.Dir + upcktimePath)
	nextTime := readTime(path)
	return nextTime
}

// SetUpdateTime writes the next update time to the state file
func (f *LocalFileCheckTrigger) SetUpdateTime() bool {
	path := f.getExecRelativeDir(f.Dir + upcktimePath)
	wait := f.CheckTimeDuration
	// Add 1 to random time since max is not included
	waitrand := time.Duration(rand.Intn(3600)) * time.Second

	return writeTime(path, time.Now().Add(wait+waitrand))
}
