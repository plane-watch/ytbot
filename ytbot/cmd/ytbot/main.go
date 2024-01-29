package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/urfave/cli/v2"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	_ "modernc.org/sqlite"
)

var (
	// App config, command line & env var configuration
	app = cli.App{
		Version: "0.0.1",
		Name:    "plane.watch youtube bot",
		Usage:   "Posts new aviation related videos to Discord",
		Description: `This program acts as a server for multiple stunnel-based endpoints, ` +
			`authenticates the feeder based on API key (UUID) check against atc.plane.watch, ` +
			`routes data to feed-in containers.`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "apikey",
				Usage:    "Google Cloud API Key",
				EnvVars:  []string{"YTBOT_GC_API_KEY"},
				Required: true,
			},
			&cli.PathFlag{
				Name:     "dbfile",
				Usage:    "Path to sqlite3 file for storage",
				EnvVars:  []string{"YTBOT_DBFILE"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "webhook",
				Usage:    "Discord Webhook for posting video",
				EnvVars:  []string{"YTBOT_WEBHOOK"},
				Required: true,
			},
		},
	}

	// Channels
	channelIds = map[channelName]channelId{
		"Mentour Pilot":   "UCwpHKudUkP5tNgmMdexB3ow",
		"LewDix Aviation": "UCPiPmwDammRsj7ZIzKyc74A",
	}
)

type (
	channelName string
	channelId   string
)

func main() {

	// set action when run
	app.Action = runApp

	// set up logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.UnixDate})
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	// run & final exit
	err := app.Run(os.Args)
	if err != nil {
		// log.Err(err).Msg("finished with error")
		os.Exit(1)
	} else {
		// log.Info().Msg("finished without error")
	}

}

func runApp(cliContext *cli.Context) error {

	log.Info().Msg("started")

	// open database
	log := log.With().Str("db", cliContext.Path("dbfile")).Logger()
	log.Debug().Msg("opening sqlite database")
	db, err := sql.Open("sqlite", cliContext.Path("dbfile"))
	if err != nil {
		return err
	}
	defer db.Close()

	// create table if required
	log.Debug().Msg("creating videos_posted table if required")
	_, err = db.Exec(
		`CREATE TABLE IF NOT EXISTS videos_posted (
			id TEXT PRIMARY KEY UNIQUE,
			date_posted TEXT NOT NULL
		 ) WITHOUT ROWID;`)
	if err != nil {
		fmt.Println(err)
	}

	// prep youtube connection
	ctx := context.Background()
	service, err := youtube.NewService(ctx, option.WithAPIKey(cliContext.String("apikey")))
	if err != nil {
		log.Fatal().AnErr("err", err).Msg("Error creating new YouTube client")
	}

	// for each tracked channel...
	for cN, cId := range channelIds {

		// published videos past 24 hours
		publishedAfter := time.Now().Add(-(time.Hour * 240))
		publishedAfterStr := publishedAfter.Format("2006-01-02T15:04:05Z")

		log := log.With().
			Str("channel_name", string(cN)).
			Str("channel_id", string(cId)).
			Time("cutoff_date", publishedAfter).
			Logger()

		log.Info().Msg("checking for new videos")

		// Make the API call to YouTube.
		call := service.Search.List([]string{"snippet"}).
			MaxResults(1).ChannelId(string(cId)).ChannelType("any").Order("date").Type("video").PublishedAfter(publishedAfterStr)
		response, err := call.Do()
		if err != nil {
			panic(err)
		}

		// Iterate through each item
		for _, item := range response.Items {

			log := log.With().
				Str("kind", item.Id.Kind).
				Str("video_id", item.Id.VideoId).
				Str("title", item.Snippet.Title).
				Logger()

			// If item is a video
			if item.Id.Kind == "youtube#video" {

				// check if item has already been posted
				r, err := db.Query(`SELECT * FROM videos_posted WHERE id=?;`, item.Id.VideoId)
				if err != nil {
					panic(err)
				}
				if r.Next() {
					log.Debug().Msg("item already posted")

				} else {

					// post video
					log.Debug().Msg("posting item")

					// webhook here
					data := fmt.Sprintf(`{"content": "New video from **%s**: %s\nhttps://youtu.be/%s"}`, item.Snippet.ChannelTitle, item.Snippet.Title, item.Id.VideoId)
					fmt.Println(data)
					whReq, err := http.NewRequest("POST", cliContext.String("webhook"), bytes.NewReader([]byte(data)))
					if err != nil {
						log.Fatal().AnErr("err", err).Msg("error preparing http request")
					}
					whReq.Header.Set("Content-Type", "application/json")
					whClient := http.Client{
						Timeout: 30 * time.Second,
					}
					whRes, err := whClient.Do(whReq)
					if err != nil {
						log.Fatal().AnErr("err", err).Msg("error preparing http request")
					}
					if whRes.StatusCode != http.StatusNoContent {
						log.Error().Str("status", whRes.Status).Msg("unexpected http response code")
					}

					// put in db
					_, err = db.Query(`INSERT INTO videos_posted (id, date_posted) VALUES (?, datetime('now'));`, item.Id.VideoId)
					if err != nil {
						log.Fatal().AnErr("err", err).Msg("error inserting video into db")
					}

				}
				err = r.Close()
				if err != nil {
					log.Fatal().AnErr("err", err).Msg("error closing rows after SELECT")
				}

			} else {
				log.Debug().Msg("skipping as item is not video")
			}
			time.Sleep(time.Second * 10)
		}
	}

	// clean up database
	log.Debug().Msg("cleaning db")
	_, err = db.Exec(`DELETE FROM videos_posted WHERE date_posted < datetime('now','-30 days');`)
	if err != nil {
		log.Fatal().AnErr("err", err).Msg("error deleting old records from db")
	}
	_, err = db.Exec(`VACUUM;`)
	if err != nil {
		log.Fatal().AnErr("err", err).Msg("error vacuuming db")
	}

	return nil
}
