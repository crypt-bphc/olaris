package db

import (
	"fmt"

	"github.com/jinzhu/gorm"
)

// BaseItem holds information that is shared between various mediatypes.
type BaseItem struct {
	UUIDable
	Overview     string
	BackdropPath string
	PosterPath   string
}

// Series holds metadata information about series.
type Series struct {
	BaseItem
	gorm.Model
	Name         string
	FirstAirDate string
	FirstAirYear uint64
	OriginalName string
	Status       string
	TmdbID       int
	Type         string
	Seasons      []*Season
}

// Season holds metadata information about seasons.
type Season struct {
	BaseItem
	gorm.Model
	Name         string
	AirDate      string
	SeasonNumber int
	Series       *Series
	SeriesID     uint
	TmdbID       int
	Episodes     []*Episode
}

// Episode holds metadata information about episodes.
type Episode struct {
	gorm.Model
	BaseItem
	Name         string
	SeasonNum    int
	EpisodeNum   int
	SeasonID     uint
	TmdbID       int
	AirDate      string
	StillPath    string
	Season       *Season
	PlayState    PlayState `gorm:"polymorphic:Owner;"`
	EpisodeFiles []EpisodeFile
}

// TimeStamp returns a unix timestamp for the given episode.
func (ep *Episode) TimeStamp() int64 {
	return ep.CreatedAt.Unix()
}

// EpisodeFile holds filesystem information about a given episode file.
type EpisodeFile struct {
	gorm.Model
	MediaItem
	EpisodeID uint
	Episode   *Episode
	Streams   []Stream `gorm:"polymorphic:Owner;"`
}

// IsSingleFile returns true if this is the only file for the given episode.
func (file *EpisodeFile) IsSingleFile() bool {
	count := 0
	db.Model(&EpisodeFile{}).Where("episode_id = ?", file.EpisodeID).Count(&count)
	if count <= 1 {
		return true
	}
	return false
}

// DeleteSelfAndMD deletes the episode file and any stale metadata information that might have resulted.
func (file *EpisodeFile) DeleteSelfAndMD() {
	// Delete all stream information
	db.Delete(Stream{}, "owner_id = ? AND owner_type = 'episode_files'", &file.ID)

	var episode Episode
	db.First(&episode, file.EpisodeID)

	if file.IsSingleFile() {
		// Delete all PlayState information
		db.Delete(PlayState{}, "owner_id = ? AND owner_type = 'episode_files'", file.EpisodeID)

		// Delete Episode
		db.Delete(&episode)

		count := 0
		var season Season
		db.First(&season, episode.SeasonID)

		db.Model(Episode{}).Where("season_id = ?", season.ID).Count(&count)

		fmt.Println(count)
		// If there are no more episodes to this season, delete the season.
		if count == 0 {
			db.Delete(Season{}, "id = ?", episode.SeasonID)
		}

		// If there are no more seasons to this series, delete it.
		count = 0
		db.Model(Season{}).Where("series_id = ?", season.SeriesID).Count(&count)
		if count == 0 {
			db.Delete(Series{}, "id = ?", season.SeriesID)
		}
	}

	// Delete all file information
	db.Delete(&file)

}

// CollectEpisodeData collects all relevant information for the given episode such as streams and playstates.
func CollectEpisodeData(episodes []Episode, userID uint) {
	for i := range episodes {
		db.Model(episodes[i]).Preload("Streams").Association("EpisodeFiles").Find(&episodes[i].EpisodeFiles)
		db.Model(episodes[i]).Where("user_id = ? AND owner_id = ? and owner_type =?", userID, episodes[i].ID, "tv_episodes").First(&episodes[i].PlayState)
	}
}

type countResult struct {
	Count uint
}

// UnwatchedEpisodesInSeriesCount retrieves the amount of unwatched episodes in a given series.
func UnwatchedEpisodesInSeriesCount(seriesID uint, userID uint) uint {
	var res countResult
	db.Raw("SELECT COUNT(*) as count FROM episodes WHERE season_id IN(SELECT id FROM seasons WHERE series_id = ?) AND id NOT IN(SELECT owner_id FROM play_states WHERE owner_type = 'episodes' AND finished = 1 AND user_id = ? AND owner_id IN(SELECT id FROM episodes WHERE season_id IN(SELECT id FROM seasons WHERE series_id = ?)))", seriesID, userID, seriesID).Scan(&res)
	return res.Count
}

// UnwatchedEpisodesInSeasonCount retrieves the amount of unwatched episodes in a given season.
func UnwatchedEpisodesInSeasonCount(seasonID uint, userID uint) uint {
	var res countResult
	db.Raw("select count(*) as count from episodes where season_id = ? AND id NOT IN (SELECT owner_id FROM play_states WHERE owner_type = 'episodes' AND finished = 1 AND user_id = ? AND owner_id IN( SELECT id FROM episodes WHERE season_id = ?))", seasonID, userID, seasonID).Scan(&res)
	return res.Count
}

// FindAllSeries retrieves all identified series from the db.
func FindAllSeries() (series []Series) {
	db.Preload("Seasons.Episodes.EpisodeFiles.Streams").Where("tmdb_id != 0").Find(&series)
	return series
}

// FindAllUnidentifiedSeries finds all episodes without any metadata information
func FindAllUnidentifiedSeries() (series []Series) {
	db.Where("tmdb_id = 0").Find(&series)
	return series
}

// FindAllUnidentifiedEpisodes finds all episodes without any metadata information
func FindAllUnidentifiedEpisodes() (episodes []Episode) {
	db.Where("tmdb_id = 0").Find(&episodes)
	return episodes
}

// SearchSeriesByTitle searches for series based on their name.
func SearchSeriesByTitle(userID uint, name string) (series []Series) {
	db.Preload("Seasons.Episodes.EpisodeFiles.Streams").Where("name LIKE ?", "%"+name+"%").Find(&series)
	return series
}

// FindSerie finds a serie by it's ID.
func FindSerie(seriesID uint) (series Series) {
	db.Where("id = ?", seriesID).Find(&series)
	return series
}

// FindSeriesByUUID retrives a serie based on it's UUID.
func FindSeriesByUUID(uuid *string) (series []Series) {
	db.Preload("Seasons.Episodes.EpisodeFiles.Streams").Where("uuid = ?", uuid).Find(&series)
	return series
}

// FindAllUnidentifiedSeasons finds all seasons without any metadata.
func FindAllUnidentifiedSeasons() (seasons []Season) {
	db.Where("tmdb_id = ?", 0).Find(&seasons)
	return seasons
}

// FindSeasonsForSeries retrieves all season for the given series based on it's UUID.
func FindSeasonsForSeries(seriesID uint) (seasons []Season) {
	db.Preload("Episodes.EpisodeFiles.Streams").Where("series_id = ?", seriesID).Find(&seasons)
	return seasons
}

// FindEpisodesForSeason finds all episodes for the given season UUID.
func FindEpisodesForSeason(seasonID uint, userID uint) (episodes []Episode) {
	db.Preload("EpisodeFiles.Streams").Where("season_id = ?", seasonID).Find(&episodes)
	CollectEpisodeData(episodes, userID)

	return episodes
}

// FindEpisodesInLibrary returns all episodes in the given library.
func FindEpisodesInLibrary(libraryID uint, userID uint) (episodes []Episode) {
	db.Where("library_id =?", libraryID).Find(&episodes)
	CollectEpisodeData(episodes, userID)

	return episodes
}

// FindPlaystateForEpisode returns the playstate for the given episode
func FindPlaystateForEpisode(episodeID uint, userID uint) (ps PlayState) {
	db.Where("user_id = ? AND owner_id = ? and owner_type =?", userID, episodeID, "episodes").First(&ps)
	return ps
}

// FindSeasonByUUID finds the season based on it's UUID.
func FindSeasonByUUID(uuid *string) (season Season) {
	db.Where("uuid = ?", uuid).Find(&season)
	return season
}

// FindSeason finds a season by it's ID
func FindSeason(seasonID uint) (season Season) {
	db.Where("id = ?", seasonID).Find(&season)
	return season
}

// FindEpisodeByUUID finds a episode based on it's UUID.
func FindEpisodeByUUID(uuid *string, userID uint) (episode *Episode) {
	var episodes []Episode
	db.Where("uuid = ?", uuid).First(&episodes)
	if len(episodes) == 1 {
		episode = &episodes[0]
		db.Model(&episode).Preload("Streams").Association("EpisodeFiles").Find(&episode.EpisodeFiles)
		db.Model(&episode).Where("user_id = ? AND owner_id = ? and owner_type =?", userID, episode.ID, "episodes").First(&episode.PlayState)
	}
	return episode
}

// FindAllEpisodeFiles retrieves all episodefiles from the db.
func FindAllEpisodeFiles() (files []EpisodeFile) {
	db.Find(&files)

	return files
}

// DeleteEpisodesFromLibrary deletes all episodes from the given library.
func DeleteEpisodesFromLibrary(libraryID uint) {
	files := []EpisodeFile{}
	db.Where("library_id = ?", libraryID).Find(&files)
	for _, file := range files {
		file.DeleteSelfAndMD()
	}
}

// EpisodeFileExists checks whether there already is a EpisodeFile present with the given path.
func EpisodeFileExists(filePath string) bool {
	count := 0
	db.Where("file_path= ?", filePath).Find(&EpisodeFile{}).Count(&count)
	if count == 0 {
		return false
	}
	return true
}

// FindSeries finds a series by it's ID
func FindSeries(seriesID uint) (series Series) {
	db.Where("id = ?", seriesID).Find(&series)
	return series
}

// CreateSeries persists a series in the database.
func CreateSeries(series *Series) {
	db.Create(series)
}

// UpdateSeries updates a series in the database.
func UpdateSeries(series *Series) {
	db.Save(series)
}

// UpdateSeason updates a season in the database.
func UpdateSeason(season *Season) {
	db.Save(season)
}

// UpdateEpisode updates an episode in the database.
func UpdateEpisode(episode *Episode) {
	db.Save(episode)
}

// UpdateEpisodeFile updates an episodeFile in the database.
func UpdateEpisodeFile(file *EpisodeFile) {
	db.Save(file)
}

// FirstOrCreateSeries returns the first instance or writes a series to the db.
func FirstOrCreateSeries(series *Series, seriesDef Series) {
	db.FirstOrCreate(series, seriesDef)
}

// FirstOrCreateEpisode returns the first instance or writes a episodes to the db.
func FirstOrCreateEpisode(episode *Episode, episodeDef Episode) {
	db.FirstOrCreate(episode, episodeDef)
}

// FirstOrCreateSeason returns the first instance or writes a episodes to the db.
func FirstOrCreateSeason(season *Season, seasonDef Season) {
	db.FirstOrCreate(season, seasonDef)
}