// Convenience wrapper around ffmpeg as a transcoder to DASH chunks
// https://github.com/go-cmd/cmd/blob/master/cmd.go was very useful while writing this module.
package ffmpeg

import (
	"os/exec"
	"os"
	"strconv"
	"path"
	"syscall"
	"io/ioutil"
	"regexp"
	"path/filepath"
	"log"
)

type TranscodingSession struct {
	cmd *exec.Cmd
	outputDir string
	// ffmpeg always starts with segment 1. However, when we start at an offset in time, we
	segmentOffset int
}

// TranscodeAndSegment starts a new ffmpeg transcode process with the given settings.
// It returns the process that was started and any error it encountered while starting it.

// TODO(Leon Handreke): Add argument to pass target codec settings in. For now, it will just copy
func NewTranscodingSession(inputPath string, outputDirBase string, startDuration int64, segmentOffset int)(*TranscodingSession, error) {

	outputDir, err := ioutil.TempDir(outputDirBase, "transcoding-session-")
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-c", "copy",
		"-f", "dash",
		"-ss", strconv.FormatInt(startDuration, 10),
		"-min_seg_duration", "5000000",
		"-media_seg_name", "segment_$Number$.m4s",
		// We serve our own manifest, so we don't really care about this.
		path.Join(outputDir, "generated_by_ffmpeg.mpd"))
	log.Println("ffmpeg started with %s", cmd.Args)
	cmd.Stderr, _ = os.Open(os.DevNull)
	cmd.Stdout = os.Stdout

	return &TranscodingSession{cmd: cmd, outputDir: outputDir, segmentOffset: segmentOffset}, nil


}

func (s *TranscodingSession) Start() error {
	return s.cmd.Start()
}

func (s *TranscodingSession) Destroy() error {
	// Signal the process group (-pid), not just the process, so that the process
	// and all its children are signaled. Else, child procs can keep running and
	// keep the stdout/stderr fd open and cause cmd.Wait to hang.
	syscall.Kill(-s.cmd.Process.Pid, syscall.SIGTERM)
	// No error handling, we don't care if ffmpeg errors out, we're done here anyway.
	s.cmd.Wait()

	err := os.RemoveAll(s.outputDir)
	if err != nil {
		return err
	}

	return nil
}

func (s *TranscodingSession) AvailableSegments() (map[int]string, error) {
	res := make(map[int]string)

	files, err := ioutil.ReadDir(s.outputDir)
	if err != nil {
		return nil, err
	}

	r := regexp.MustCompile("segment_(?P<number>\\d+).m4s")

	for _, f := range files {
		match := r.FindString(f.Name())
		if match != "" {
			segmentFsNumber, _ := strconv.Atoi(match[len("segment_"):len(match)-len(".m4s")])
			res[segmentFsNumber + s.segmentOffset] = filepath.Join(s.outputDir, f.Name())
		}

	}

	return res, nil
}