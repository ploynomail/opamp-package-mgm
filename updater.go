package opamppackagemgm

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/kr/binarydist"
)

type Updater struct {
	ctx                context.Context       // Context to use for the updater
	CurrentVersion     string                // Currently running version. `dev` is a special version here and will cause the updater to never update.
	Requester          Requester             // Optional parameter to override existing HTTP request handler
	Dir                string                // Directory to store selfupdate state.
	Trigger            TriggerUpdater        // Optional parameter to override existing trigger handler
	Logger             Loggerr               // Optional parameter to override existing logger
	Info               UpdatePackageInfo     // Info about the update
	OnSuccessfulUpdate func(context.Context) // Optional function to run after an update has successfully taken place
	OnFailedUpdate     func(context.Context) // Optional function to run after an update has failed
	IsGzipped          bool                  // Optional parameter to specify if the binary is gzipped
}

// BackgroundRun 开始更新检查和应用周期。
func (u *Updater) BackgroundRun() error {
	if err := os.MkdirAll(u.getExecRelativeDir(u.Dir), 0755); err != nil {
		// fail
		return err
	}
	if u.Trigger == nil {
		// fail
		return fmt.Errorf("trigger is nil")
	}
	if u.Logger == nil {
		u.Logger = NewLog()
	}
	if u.Requester == nil {
		u.Requester = defaultHTTPRequester
	}
	for {
		select {
		case info := <-u.WantUpdate():
			u.Info = info
			if err := canUpdate(); err != nil {
				// fail
				return err
			}
			if err := u.Update(); err != nil {
				return err
			}
		case <-u.ctx.Done():
			return nil
		}
	}
}

func (u *Updater) WantUpdate() chan UpdatePackageInfo {
	return u.Trigger.Trigger(u.ctx)
}

// canUpdate 检查更新条件是否满足。
func canUpdate() (err error) {
	// get the directory the file exists in
	path, err := os.Executable()
	if err != nil {
		return
	}

	fileDir := filepath.Dir(path)
	fileName := filepath.Base(path)

	// 尝试打开文件目录中的文件
	newPath := filepath.Join(fileDir, fmt.Sprintf(".%s.new", fileName))
	fp, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return
	}
	fp.Close()

	_ = os.Remove(newPath)
	return
}

// Update initiates the self update process
func (u *Updater) Update() error {
	path, err := os.Executable()
	if err != nil {
		return err
	}

	if resolvedPath, err := filepath.EvalSymlinks(path); err == nil {
		path = resolvedPath
	}

	// we are on the latest version, nothing to do
	if u.Info.Version == u.CurrentVersion {
		return nil
	}

	old, err := os.Open(path)
	if err != nil {
		return err
	}
	defer old.Close()

	bin, err := u.fetchAndVerifyPatch(old)
	if err != nil {
		if err == ErrHashMismatch {
			log.Println("update: hash mismatch from patched binary")
		}

		// if patch failed grab the full new bin
		bin, err = u.fetchAndVerifyFullBin()
		if err != nil {
			if err == ErrHashMismatch {
				log.Println("update: hash mismatch from full binary")
			} else {
				log.Println("update: fetching full binary,", err)
			}
			return err
		}
	}
	// close the old binary before installing because on windows
	// it can't be renamed if a handle to the file is still open
	old.Close()

	err, errRecover := fromStream(bytes.NewBuffer(bin))
	if errRecover != nil {
		return fmt.Errorf("update and recovery errors: %q %q", err, errRecover)
	}
	if err != nil {
		return err
	}

	// update was successful, run func if set
	if u.OnSuccessfulUpdate != nil {
		u.OnSuccessfulUpdate(u.ctx)
	}

	return nil
}

func fromStream(updateWith io.Reader) (err error, errRecover error) {
	updatePath, err := os.Executable()
	if err != nil {
		return
	}

	var newBytes []byte
	newBytes, err = ioutil.ReadAll(updateWith)
	if err != nil {
		return
	}

	// get the directory the executable exists in
	updateDir := filepath.Dir(updatePath)
	filename := filepath.Base(updatePath)

	// Copy the contents of of newbinary to a the new executable file
	newPath := filepath.Join(updateDir, fmt.Sprintf(".%s.new", filename))
	fp, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return
	}
	defer fp.Close()
	_, err = io.Copy(fp, bytes.NewReader(newBytes))

	// if we don't call fp.Close(), windows won't let us move the new executable
	// because the file will still be "in use"
	fp.Close()

	// this is where we'll move the executable to so that we can swap in the updated replacement
	oldPath := filepath.Join(updateDir, fmt.Sprintf(".%s.old", filename))

	// delete any existing old exec file - this is necessary on Windows for two reasons:
	// 1. after a successful update, Windows can't remove the .old file because the process is still running
	// 2. windows rename operations fail if the destination file already exists
	_ = os.Remove(oldPath)

	// move the existing executable to a new file in the same directory
	err = os.Rename(updatePath, oldPath)
	if err != nil {
		return
	}

	// move the new exectuable in to become the new program
	err = os.Rename(newPath, updatePath)

	if err != nil {
		// copy unsuccessful
		errRecover = os.Rename(oldPath, updatePath)
	} else {
		// copy successful, remove the old binary
		errRemove := os.Remove(oldPath)

		// windows has trouble with removing old binaries, so hide it instead
		if errRemove != nil {
			_ = hideFile(oldPath)
		}
	}

	return
}

func (u *Updater) getExecRelativeDir(dir string) string {
	filename, _ := os.Executable()
	path := filepath.Join(filepath.Dir(filename), dir)
	return path
}

func (u *Updater) fetchAndVerifyPatch(old io.Reader) ([]byte, error) {
	bin, err := u.fetchAndApplyPatch(old)
	if err != nil {
		return nil, err
	}
	if !verifySha(bin, u.Info.ContentHash) {
		return nil, ErrHashMismatch
	}
	return bin, nil
}

func (u *Updater) fetchAndVerifyFullBin() ([]byte, error) {
	bin, err := u.fetchBin()
	if err != nil {
		return nil, err
	}
	verified := verifySha(bin, u.Info.ContentHash)
	if !verified {
		return nil, ErrHashMismatch
	}
	return bin, nil
}

func (u *Updater) fetchAndApplyPatch(old io.Reader) ([]byte, error) {
	r, err := u.fetch(u.Info.DownloadUrl + ".patch")
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var buf bytes.Buffer
	err = binarydist.Patch(old, &buf, r)
	return buf.Bytes(), err
}

func (u *Updater) fetchBin() ([]byte, error) {
	r, err := u.fetch(u.Info.DownloadUrl)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	buf := new(bytes.Buffer)
	if u.IsGzipped {
		gz, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		if _, err = io.Copy(buf, gz); err != nil {
			return nil, err
		}
	} else {
		if _, err = io.Copy(buf, r); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func (u *Updater) fetch(url string) (io.ReadCloser, error) {
	if u.Requester == nil {
		return defaultHTTPRequester.Fetch(url)
	}

	readCloser, err := u.Requester.Fetch(url)
	if err != nil {
		return nil, err
	}

	if readCloser == nil {
		return nil, fmt.Errorf("Fetch was expected to return non-nil ReadCloser")
	}

	return readCloser, nil
}
