package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/kr/binarydist"
)

var version, genDir, patchWith string

type current struct {
	Version string
	Sha256  []byte
	IsPatch bool
}

func generateSha256(path string) []byte {
	h := sha256.New()
	b, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println(err)
	}
	h.Write(b)
	sum := h.Sum(nil)
	return sum
	//return base64.URLEncoding.EncodeToString(sum)
}

type gzReader struct {
	z, r io.ReadCloser
}

func (g *gzReader) Read(p []byte) (int, error) {
	return g.z.Read(p)
}

func (g *gzReader) Close() error {
	g.z.Close()
	return g.r.Close()
}

func newGzReader(r io.ReadCloser) io.ReadCloser {
	var err error
	g := new(gzReader)
	g.r = r
	g.z, err = gzip.NewReader(r)
	if err != nil {
		panic(err)
	}
	return g
}

func createUpdate(path string, platform string) {
	c := current{Version: version, Sha256: generateSha256(path), IsPatch: patchWith != ""}

	b, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		fmt.Println("error:", err)
	}
	fileName := filepath.Base(path)
	os.MkdirAll(filepath.Join(genDir, fileName, version), 0755)
	err = os.WriteFile(filepath.Join(genDir, fileName, platform+".json"), b, 0755)
	if err != nil {
		panic(err)
	}
	if patchWith == "" {
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		f, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		w.Write(f)
		w.Close() // You must close this first to flush the bytes to the buffer.

		err = os.WriteFile(filepath.Join(genDir, fileName, version, platform+".gz"), buf.Bytes(), 0755)
		if err != nil {
			panic(err)
		}
	} else {
		files, err := os.ReadDir(filepath.Join(genDir, fileName))
		if err != nil {
			fmt.Println(err)
		}

		for _, file := range files {
			if !file.IsDir() {
				continue
			}

			if file.Name() != patchWith {
				continue
			}
			os.Mkdir(filepath.Join(genDir, file.Name(), version), 0755)

			fName := filepath.Join(genDir, fileName, patchWith, platform+".gz")
			old, err := os.Open(fName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Can't open %s: error: %s\n", fName, err)
				// Don't have an old release for this os/arch, continue on
				continue
			}

			// fName = filepath.Join(genDir, file.Name(), version, platform+".gz")
			newF, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Can't open %s: error: %s\n", fName, err)
				os.Exit(1)
			}

			ar := newGzReader(old)
			defer ar.Close()
			patch := new(bytes.Buffer)
			if err := binarydist.Diff(ar, newF, patch); err != nil {
				panic(err)
			}
			err = os.WriteFile(filepath.Join(genDir, fileName, version, platform+".patch"), patch.Bytes(), 0755)
			if err != nil {
				panic(err)
			}
		}
	}
}

func printUsage() {
	fmt.Println("")
	fmt.Println("Positional arguments:")
	fmt.Println("\tSingle platform: go-selfupdate myapp 1.2")
	fmt.Println("\tCross platform: go-selfupdate /tmp/mybinares/ 1.2")
}

func createBuildDir() {
	os.MkdirAll(genDir, 0755)
}

func main() {
	outputDirFlag := flag.String("o", "public", "Output directory for writing updates")
	patch := flag.String("patch", "", "Create a patch file from the given version")
	var defaultPlatform string
	goos := os.Getenv("GOOS")
	goarch := os.Getenv("GOARCH")
	if goos != "" && goarch != "" {
		defaultPlatform = goos + "-" + goarch
	} else {
		defaultPlatform = runtime.GOOS + "-" + runtime.GOARCH
	}
	platformFlag := flag.String("platform", defaultPlatform,
		"Target platform in the form OS-ARCH. Defaults to running os/arch or the combination of the environment variables GOOS and GOARCH if both are set.")

	flag.Parse()
	if flag.NArg() < 2 {
		flag.Usage()
		printUsage()
		os.Exit(0)
	}

	platform := *platformFlag
	appPath := flag.Arg(0)
	version = flag.Arg(1)
	genDir = *outputDirFlag
	patchWith = *patch

	createBuildDir()

	// If dir is given create update for each file
	fi, err := os.Stat(appPath)
	if err != nil {
		panic(err)
	}

	if fi.IsDir() {
		files, err := os.ReadDir(appPath)
		if err == nil {
			for _, file := range files {
				createUpdate(filepath.Join(appPath, file.Name()), file.Name())
			}
			os.Exit(0)
		}
	}
	createUpdate(appPath, platform)
}
