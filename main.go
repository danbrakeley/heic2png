package main

import (
	"flag"
	"fmt"
	"image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/adrium/goheif"
	"github.com/danbrakeley/frog"
)

// (go tool nm) -ldflags '-X "main.Version=${{version}}"'
var Version string = "unknown"

// (go tool nm) -ldflags '-X "main.BuildTimestamp=${{date -u +"%Y-%m-%dT%H:%M:%SZ"}}"'
var BuildTimestamp string = "unknown"

func main() {
	// this one line main() ensures os.Exit is only called after all defers have run
	os.Exit(main_())
}

func main_() int {
	var findAll bool
	var singleFile string
	var numWorkers int
	var deleteOnSuccess bool
	var forceOverwrite bool
	var version bool
	flag.BoolVar(&findAll, "all", false, "")
	flag.BoolVar(&findAll, "a", false, "")
	flag.BoolVar(&deleteOnSuccess, "delete", false, "")
	flag.StringVar(&singleFile, "file", "", "")
	flag.StringVar(&singleFile, "f", "", "")
	flag.BoolVar(&forceOverwrite, "overwrite", false, "")
	flag.IntVar(&numWorkers, "procs", runtime.NumCPU(), "")
	flag.IntVar(&numWorkers, "p", runtime.NumCPU(), "")
	flag.BoolVar(&version, "version", false, "")
	flag.BoolVar(&version, "v", false, "")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage of heic2png:
  -a, --all     · · · · ·   find all *.heic files and convert them to *.png
      --delete  · · · · ·   delete original heic file after successfull conversion
  -f, --file    <filename>  specify a single heic file to convert to .png
      --overwrite · · · ·   if target png file already exists, overwrite it
  -p, --procs   <num_procs> max number of files to process in parallel (default %d)
  -v, --version · · · · ·   print version information and exit
  -h, --help    · · · · ·   print this message and exit
`, runtime.NumCPU())
	}
	flag.Parse()

	// print version info and exit
	if version {
		fmt.Printf("Version: %s\nBuilt On: %s\n", Version, BuildTimestamp)
		return 0
	}

	if !findAll && len(singleFile) == 0 {
		fmt.Fprintf(os.Stderr, "no file(s) specified\n")
		flag.Usage()
		return -1
	}

	log := frog.New(frog.Auto, frog.POFieldIndent(30))
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
			if v.IsDir() {
				continue
			}
			name := v.Name()
			if !strings.HasSuffix(strings.ToLower(filepath.Ext(name)), ".heic") {
				continue
			}
			filelist = append(filelist, v.Name())
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
	log.Info("starting", frog.Int("num_workers", numWorkers), frog.String("version", Version), frog.String("built_on", BuildTimestamp))
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func(thread int) {
			defer wg.Done()
			l := frog.AddAnchor(log)
			defer frog.RemoveAnchor(l)

			// loop until channel is closed
			for t := range chTasks {
				fid := frog.Int("worker_id", thread)
				fheic := frog.String("heic_path", t.HeicPath)
				fpng := frog.String("png_path", t.PngPath)

				err := convertHeicToPng(t.HeicPath, t.PngPath, forceOverwrite, func(step, max int) {
					l.Transient("converting: "+progressBar(step, max, '·', '☆'), fheic, fpng, fid)
				})
				if err != nil {
					l.Error("conversion failed", fheic, fpng, frog.Err(err), fid)
					atomic.AddInt32(&numErrs, 1)
					continue
				}

				l.Info("image converted", fheic, fpng, fid)

				if deleteOnSuccess {
					l.Transient("deleting", fheic, fid)

					if err = os.Remove(t.HeicPath); err != nil {
						l.Error("delete failed", fheic, frog.Err(err), fid)
						atomic.AddInt32(&numErrs, 1)
						continue
					}
					l.Info("original image deleted", fheic, fid)
				}
			}
		}(i)
	}

	// send work to workers
	for _, v := range filelist {
		chTasks <- Task{
			HeicPath: v,
			PngPath:  removeExt(v) + ".png",
		}
	}

	close(chTasks)
	wg.Wait()
	log.Info("done", frog.Int32("num_errors", numErrs))

	// use the worker error count as the exit status
	return int(numErrs)
}

type Task struct {
	HeicPath string
	PngPath  string
}

func convertHeicToPng(filenameIn, filenameOut string, forceOverwrite bool, fnProgress func(step, max int)) error {
	if fnProgress == nil {
		fnProgress = func(_, _ int) {}
	}

	fnProgress(0, 4)
	fIn, err := os.Open(filenameIn)
	if err != nil {
		return fmt.Errorf("unable to open %s: %w", filenameIn, err)
	}
	defer fIn.Close()

	fnProgress(1, 4)
	img, err := goheif.Decode(fIn)
	if err != nil {
		return fmt.Errorf("unable to decode %s: %w", filenameIn, err)
	}

	fnProgress(2, 4)
	var fOut *os.File
	if forceOverwrite {
		fOut, err = os.Create(filenameOut)
	} else {
		fOut, err = os.OpenFile(filenameOut, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o666)
	}
	if err != nil {
		return fmt.Errorf("unable to create %s: %v", filenameOut, err)
	}
	defer fOut.Close()

	fnProgress(3, 4)
	pngenc := png.Encoder{CompressionLevel: png.BestSpeed}
	err = pngenc.Encode(fOut, img)
	if err != nil {
		return fmt.Errorf("unable to encode %s: %w", filenameOut, err)
	}

	fnProgress(4, 4)
	return nil
}

func progressBar(cur, max int, empty, full rune) string {
	var sb strings.Builder
	sb.Grow(max * 4)
	for i := 0; i < max; i++ {
		if i < cur {
			sb.WriteRune(full)
		} else {
			sb.WriteRune(empty)
		}
	}
	return sb.String()
}

func removeExt(p string) string {
	return strings.TrimSuffix(p, filepath.Ext(p))
}
