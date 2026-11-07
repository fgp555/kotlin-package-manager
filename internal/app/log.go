package app

import (
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"time"
)

func runLog(args []string) error {
	flags := flag.NewFlagSet("kpm log", flag.ContinueOnError)
	lines := flags.Int("n", 30, "number of lines to show")
	flags.IntVar(lines, "lines", 30, "alias for -n")
	follow := flags.Bool("f", false, "follow new log output")
	flags.BoolVar(follow, "follow", false, "alias for -f")
	newOnly := flags.Bool("nf", false, "alias for -n 0 -f")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *newOnly {
		*lines, *follow = 0, true
	}
	if flags.NArg() != 0 || *lines < 0 {
		return errors.New("usage: kpm log [-n lines] [-f] or kpm log -nf; lines must be non-negative")
	}

	p, err := findProject()
	if err != nil {
		return err
	}
	if !p.gradle {
		return errors.New("kpm log reads the Gradle Compose Hot Reload log; Kotlin Toolchain reports reloads in the run terminal")
	}
	path, err := findHotReloadLog(p.root)
	if err != nil {
		return err
	}
	return readHotReloadLog(path, *lines, *follow)
}

var errLogNotFound = errors.New("Compose Hot Reload log not found; start kpm run dev first")

func hotReloadLogPaths(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	const logName = "build/run/main/main.chr.log"
	paths := []string{filepath.Join(root, logName)}
	for _, entry := range entries {
		if entry.IsDir() {
			paths = append(paths, filepath.Join(root, entry.Name(), logName))
		}
	}
	return paths, nil
}

func findHotReloadLog(root string) (string, error) {
	paths, err := hotReloadLogPaths(root)
	if err != nil {
		return "", err
	}
	var newest time.Time
	var path string
	for _, candidate := range paths {
		info, err := os.Stat(candidate)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode().IsRegular() && (path == "" || info.ModTime().After(newest)) {
			newest, path = info.ModTime(), candidate
		}
	}
	if path == "" {
		return "", errLogNotFound
	}
	return path, nil
}

type logCursor struct {
	path   string
	info   os.FileInfo
	offset int64
}

func (cursor *logCursor) drain() error {
	info, err := os.Stat(cursor.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if os.SameFile(cursor.info, info) && info.Size() == cursor.offset {
		return nil
	}
	file, err := os.Open(cursor.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	info, err = file.Stat()
	if err != nil {
		file.Close()
		return err
	}
	if !os.SameFile(cursor.info, info) || info.Size() < cursor.offset {
		cursor.offset = 0
	}
	written, err := copyLogFrom(file, cursor.offset)
	cursor.info, cursor.offset = info, cursor.offset+written
	return err
}

func readHotReloadLog(path string, lines int, follow bool) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}
	offset, err := tailOffset(file, info.Size(), lines)
	if err != nil {
		file.Close()
		return err
	}
	written, err := copyLogFrom(file, offset)
	if err != nil || !follow {
		return err
	}
	cursor := logCursor{path: path, info: info, offset: offset + written}
	return followLog(nil, cursor.drain)
}

func followLog(done <-chan struct{}, drain func() error) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := drain(); err != nil {
				return err
			}
		case <-done:
			return drain()
		}
	}
}

func copyLogFrom(file *os.File, offset int64) (int64, error) {
	defer file.Close()
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return 0, err
	}
	return io.Copy(os.Stdout, file)
}

// Scan backwards in blocks so tailing a large log only reads the required suffix.
func tailOffset(file io.ReaderAt, size int64, lines int) (int64, error) {
	if lines == 0 {
		return size, nil
	}
	var buffer [8192]byte
	for end := size; end > 0; {
		start := max(0, end-int64(len(buffer)))
		chunk := buffer[:end-start]
		if _, err := file.ReadAt(chunk, start); err != nil {
			return 0, err
		}
		for i := len(chunk) - 1; i >= 0; i-- {
			position := start + int64(i)
			if chunk[i] == '\n' && position != size-1 {
				lines--
				if lines == 0 {
					return position + 1, nil
				}
			}
		}
		end = start
	}
	return 0, nil
}
