# heic2png

`heic2png` losslessly transcodes heic images png. It includes options to automatically batch all the .heic files it finds in a folder, and can spread the work across all available CPU cores. Currently, it only outputs png, and only with minimal compression to keep it fast. Note that this may result in relatively large png files, which is preferrable to spending a bunch of time trying to compress image data that likely won't compress very well anyway.

## Install

Grab the latest zip with the exe off the Releases page, then extract it into a folder in your path.

## Usage

This is a command line app, so open your favorite Windows shell/terminal/console, and pass `-h` or `--help` to see all the options available. As of v0.2.0, your output should look something like this:

```text
$ heic2png -h
Usage of heic2png:
  -a, --all     · · · · ·   find all *.heic files and convert them to *.png
      --delete  · · · · ·   delete original heic file after successfull conversion
  -f, --file    <filename>  specify a single heic file to convert to .png
      --overwrite · · · ·   if target png file already exists, overwrite it
  -p, --procs   <num_procs> max number of files to process in parallel (defaults to number of available cores)
  -v, --version · · · · ·   print version information and exit
  -h, --help    · · · · ·   print this message and exit
```

## Why?

For whatever reason, I prefer my phone to be Apple, but my desktop to be PC. These days iPhones create photos using HEIC, but Windows doesn't really have good first-party support yet. Up until now, I've been using Irfanview to losslessly batch convert HEIC files to PNG, but the problem with Irfanview is it is single-threaded, so it takes a lot longer than it should to run through a large folder.
