package main

import (
	"context"
	"log"
	"time"

	opamppackagemgm "github.com/ploynomail/opamp-package-mgm"
)

// the app's version. This will be set on build.
var version string

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// print the current version
	log.Printf("(example-updater) Hello world! I am currently version: %q", version)

	// try to update
	up := opamppackagemgm.NewUpdater(context.TODO(), version, "./tmp")
	up.WithTrigger(opamppackagemgm.NewRemoteFileCheckTrigger(
		"http://localhost:8080",
		"http://localhost:8080",
		"example-updater",
		"./tmp/",
		3*time.Second,
		opamppackagemgm.NewLog(),
	))
	up.WithLogger(opamppackagemgm.NewLog())
	up.WithIsGzipped(true)
	onSuccess := func(ctx context.Context) {
		log.Printf("(example-updater) Successfully updated to version: %q", version)
	}
	up.WithOnSuccessfulUpdate(onSuccess)
	go up.BackgroundRun()
	// print out latest version available
	time.Sleep(time.Minute * 2)
	log.Printf("(example-updater) Latest version available: %q", version)
}
