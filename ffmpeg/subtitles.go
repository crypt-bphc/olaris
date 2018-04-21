package ffmpeg

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"time"
)

func NewSubtitleSession(
	stream OfferedStream,
	outputDirBase string) (*TranscodingSession, error) {

	outputDir, err := ioutil.TempDir(outputDirBase, "subtitle-session-")
	if err != nil {
		return nil, err
	}

	extractSubtitlesCmd := exec.Command("ffmpeg",
		// -ss being before -i is important for fast seeking
		"-i", stream.MediaFilePath,
		"-map", fmt.Sprintf("0:%d", stream.StreamId),
		"-threads", "2",
		"-f", "webvtt",
		"stream0_0.m4s")
	extractSubtitlesCmd.Stderr, _ = os.Open(os.DevNull)
	extractSubtitlesCmd.Dir = outputDir

	log.Println("ffmpeg initialized with", extractSubtitlesCmd.Args)

	return &TranscodingSession{
		cmd:            extractSubtitlesCmd,
		Stream:         stream,
		outputDir:      outputDir,
		firstSegmentId: 0,
	}, nil
}

func GetOfferedSubtitleStreams(mediaFilePath string) ([]OfferedStream, error) {
	container, err := Probe(mediaFilePath)
	if err != nil {
		return nil, err
	}

	offeredStreams := []OfferedStream{}

	for _, stream := range container.Streams {
		if stream.CodecType != "subtitle" {
			continue
		}

		offeredStreams = append(offeredStreams, OfferedStream{
			StreamKey: StreamKey{
				MediaFilePath:    mediaFilePath,
				StreamId:         int64(stream.Index),
				RepresentationId: "webvtt",
			},
			TotalDuration:          container.Format.Duration(),
			StreamType:             "subtitle",
			Language:               GetLanguageTag(stream),
			Title:                  GetTitleOrHumanizedLanguage(stream),
			EnabledByDefault:       stream.Disposition["default"] != 0,
			SegmentStartTimestamps: []time.Duration{0},
		})
	}

	return offeredStreams, nil
}
