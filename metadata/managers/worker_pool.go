package managers

import (
	"github.com/Jeffail/tunny"
	log "github.com/sirupsen/logrus"
	"gitlab.com/olaris/olaris-server/metadata/agents"
	"gitlab.com/olaris/olaris-server/metadata/db"
)

// WorkerPool is a container for the various workers that a library needs
type WorkerPool struct {
	tmdbPool   *tunny.Pool
	probePool  *tunny.Pool
	Subscriber LibrarySubscriber
}

// SetSubscriber tells the pool which subscriber to send events to.
func (p *WorkerPool) SetSubscriber(s LibrarySubscriber) {
	p.Subscriber = s
}

// Shutdown properly shuts down the WP
func (p *WorkerPool) Shutdown() {
	log.Debugln("Shutting down worker pool")
	p.tmdbPool.Close()
	p.probePool.Close()
	log.Debugln("Pool shut down")
}

// NewDefaultWorkerPool needs a description
func NewDefaultWorkerPool() *WorkerPool {
	p := &WorkerPool{}
	agent := agents.NewTmdbAgent()

	// The MovieDB currently has a 40 requests per 10 seconds limit. Assuming every request takes a second then four workers is probably ideal.
	// TODO(Maran): Create a global rate limiter for TMDB here instead of relying on these
	//  estimates.
	p.tmdbPool = tunny.NewFunc(3, func(payload interface{}) interface{} {
		log.Debugln("Current TMDB queue length:", p.tmdbPool.QueueLength())
		var err error
		if episode, ok := payload.(*db.Episode); ok {
			err = processEpisodePayload(episode, agent, p.Subscriber)
		}
		if episodeFile, ok := payload.(*db.EpisodeFile); ok {
			err = processEpisodeFilePayload(episodeFile, agent, p.Subscriber)
		}
		if movie, ok := payload.(*db.Movie); ok {
			err = processMoviePayload(movie, agent, p.Subscriber)
		}
		if movieFile, ok := payload.(*db.MovieFile); ok {
			err = processMovieFilePayload(movieFile, agent, p.Subscriber)
		}

		if err != nil {
			log.WithField("error", err.Error()).
				Error("tmdbPool failed to process payload")
		}

		return nil
	})

	p.probePool = tunny.NewFunc(4, func(payload interface{}) interface{} {
		log.Println("Current Probe queue length:", p.probePool.QueueLength())
		if job, ok := payload.(*probeJob); ok {
			job.man.ProbeFile(job.node)
		} else {
			log.Warnln("Got a ProbeJob that couldn't be cast as such, refreshing library might fail.")
		}
		return nil
	})

	return p
}

func processEpisodePayload(
	episode *db.Episode,
	agent *agents.TmdbAgent,
	subscriber LibrarySubscriber) error {
	if err := UpdateEpisodeMD(episode, agent); err != nil {
		log.
			WithFields(log.Fields{"error": err}).
			Warnln("Got an error updating metadata for series.")
		return err
	}
	return nil
}

func processEpisodeFilePayload(
	episodeFile *db.EpisodeFile,
	agent *agents.TmdbAgent,
	subscriber LibrarySubscriber) error {

	_, err := GetOrCreateEpisodeForEpisodeFile(episodeFile, agent, subscriber)
	return err
}

func processMoviePayload(
	movie *db.Movie,
	agent *agents.TmdbAgent,
	subscriber LibrarySubscriber) error {

	return UpdateMovieMD(movie, agent)
}

func processMovieFilePayload(
	movieFile *db.MovieFile,
	agent *agents.TmdbAgent,
	subscriber LibrarySubscriber) error {

	_, err := GetOrCreateMovieForMovieFile(movieFile, agent, subscriber)
	return err
}
