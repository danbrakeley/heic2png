package main

import (
	"flag"
	"fmt"
	"image/png"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/danbrakeley/frog"
	"github.com/jdeng/goheif"
)

// -ldflags '-X "github.com/danbrakeley/heic2png/main.Version=${{ github.event.release.tag_name }}"'
var Version string

// -ldflags '-X "github.com/danbrakeley/heic2png/main.BuildTime=${{ github.event.release.created_at }}"'
var BuildTime string

// -ldflags '-X "github.com/danbrakeley/heic2png/main.ReleaseURL=${{ github.event.release.html_url }}"'
var ReleaseURL string

func main() {
	// this one line main() ensures os.Exit is only called after all defers have run
	os.Exit(main_())
}

func main_() int {
	var findAll bool
	var singleFile string
	var numWorkers int
	var version bool
	flag.BoolVar(&findAll, "all", false, "")
	flag.BoolVar(&findAll, "a", false, "")
	flag.StringVar(&singleFile, "file", "", "")
	flag.StringVar(&singleFile, "f", "", "")
	flag.IntVar(&numWorkers, "procs", runtime.NumCPU(), "")
	flag.IntVar(&numWorkers, "p", runtime.NumCPU(), "")
	flag.BoolVar(&version, "version", false, "")
	flag.BoolVar(&version, "v", false, "")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage of heic2png:
  -a, --all        find all *.heic files and convert them to *.png
  -f, --file       specify a single heic file to convert to .png
  -p, --procs      max number of files to process in parallel
  -v, --version    print version information and exit
`)
	}
	flag.Parse()

	// print version info and exit
	if version {
		fmt.Printf(`heic2png %s
  built on: %s
  release url: %s
		`, Version, BuildTime, ReleaseURL)
		return 0
	}

	if !findAll && len(singleFile) == 0 {
		fmt.Fprintf(os.Stderr, "no file(s) specified\n")
		flag.Usage()
		return -1
	}

	log := frog.New(frog.Auto)
	defer log.Close()

	// build list of files
	var filelist []string
	switch {
	case len(singleFile) > 0:
		filelist = append(filelist, singleFile)

	case findAll:
		// get local directory
		cwd, err := os.Getwd()
		if err != nil {
			log.Error("os.Getwd() had error", frog.Err(err))
			return -1
		}
		files, err := ioutil.ReadDir(cwd)
		if err != nil {
			log.Error("ioutil.ReadDir(cwd) had error", frog.String("cwd", cwd), frog.Err(err))
			return -1
		}
		filelist = make([]string, 0, len(files))
		for _, v := range files {
			if !v.IsDir() {
				name := v.Name()
				if strings.HasSuffix(strings.ToLower(path.Ext(name)), ".heic") {
					filelist = append(filelist, v.Name())
				}
			}
		}
	}

	if len(filelist) == 0 {
		log.Warning("no files found, nothing to do")
		return 0
	}

	// only spin up as many workers as we'll actually need
	if numWorkers > len(filelist) {
		numWorkers = len(filelist)
	}

	var wg sync.WaitGroup
	chTasks := make(chan Task)
	var numErrs int32

	// spin up workers
	log.Info("starting workers", frog.Int("count", numWorkers))
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func(thread int) {
			defer wg.Done()
			l := frog.AddAnchor(log)
			defer frog.RemoveAnchor(l)

			// loop until channel is closed
			for t := range chTasks {
				fThread := frog.Int("worker", thread)
				fFIn := frog.String("in", t.FilenameIn)
				fFOut := frog.String("out", t.FilenameOut)

				l.Transient("processing", fThread, fFIn, fFOut)
				err := convertHeicToPng(t.FilenameIn, t.FilenameOut)
				if err != nil {
					l.Error("failed", fThread, fFIn, fFOut, frog.Err(err))
					atomic.AddInt32(&numErrs, 1)
				} else {
					l.Info("processed", fThread, fFIn, fFOut)
				}
				l.Transient("idle", fThread)
			}
		}(i)
	}

	// send work to workers
	for _, v := range filelist {
		chTasks <- Task{
			FilenameIn:  v,
			FilenameOut: strings.TrimSuffix(v, path.Ext(v)) + ".png",
		}
	}

	close(chTasks)
	wg.Wait()
	log.Info("all workers stopped", frog.Int32("num_errors", numErrs))

	// use the worker error count as the exit status
	return int(numErrs)
}

type Task struct {
	FilenameIn  string
	FilenameOut string
}

func convertHeicToPng(filenameIn, filenameOut string) error {
	fIn, err := os.Open(filenameIn)
	if err != nil {
		return fmt.Errorf("unable to open %s: %w", filenameIn, err)
	}
	defer fIn.Close()

	img, err := goheif.Decode(fIn)
	if err != nil {
		return fmt.Errorf("unable to decode %s: %w", filenameIn, err)
	}

	fOut, err := os.Create(filenameOut)
	if err != nil {
		return fmt.Errorf("unable to create %s: %v", filenameOut, err)
	}
	defer fOut.Close()

	err = png.Encode(fOut, img)
	if err != nil {
		return fmt.Errorf("unable to encode %s: %w", filenameOut, err)
	}

	return nil
}
