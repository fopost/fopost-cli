package cmd

import (
	"errors"
	"strconv"

	fopost "github.com/fopost/fopost-go"
	"github.com/spf13/cobra"

	"github.com/fopost/fopost-cli/internal/output"
)

// errMissingPlaylist keeps `set-default-playlist` from silently clearing the
// default when the id is simply forgotten; --clear is the explicit way.
var errMissingPlaylist = errors.New("a playlist id is required, or pass --clear to clear the default")

// intLabel prints an optional count, or a dash when the API did not report one.
func intLabel(value *int) string {
	if value == nil {
		return "-"
	}
	return strconv.Itoa(*value)
}

// Per-network extras on an account: the boards, playlists, captions, languages
// and searches each network offers beyond publishing.

func newAccountsPinterestCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pinterest",
		Short: "List and create the boards a Pinterest account can pin to",
	}
	cmd.AddCommand(newPinterestBoardsCmd(state), newPinterestCreateBoardCmd(state))
	return cmd
}

func newPinterestBoardsCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "boards <account-id>",
		Short: "List the boards this connection can pin to",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			boards, err := client.Accounts.ListPinterestBoards(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(boards, func() {
				rows := make([][]string, 0, len(boards))
				for _, b := range boards {
					rows = append(rows, []string{b.ID, b.Name, output.Dash(deref(b.Privacy))})
				}
				printer.Table([]string{"id", "name", "privacy"}, rows)
			})
		},
	}
}

func newPinterestCreateBoardCmd(state *State) *cobra.Command {
	var description, privacy string
	cmd := &cobra.Command{
		Use:   "create-board <account-id> <name>",
		Short: "Create a board on the connected Pinterest account",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			board, err := client.Accounts.CreatePinterestBoard(cmd.Context(), args[0],
				&fopost.CreatePinterestBoardRequest{
					Name: args[1], Description: description, Privacy: privacy,
				})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(board, func() {
				printer.Table([]string{"id", "name", "privacy"},
					[][]string{{board.ID, board.Name, output.Dash(deref(board.Privacy))}})
			})
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "Board description")
	cmd.Flags().StringVar(&privacy, "privacy", "", "PUBLIC, PROTECTED or SECRET")
	return cmd
}

func newAccountsYouTubeCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "youtube",
		Short: "Manage a YouTube channel's playlists and caption tracks",
	}
	cmd.AddCommand(
		newYouTubePlaylistsCmd(state),
		newYouTubeCreatePlaylistCmd(state),
		newYouTubeSetDefaultPlaylistCmd(state),
		newYouTubeCaptionsCmd(state),
		newYouTubeTranscriptCmd(state),
	)
	return cmd
}

func newYouTubePlaylistsCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "playlists <account-id>",
		Short: "List the channel's playlists, with the stored default marked",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			playlists, err := client.Accounts.ListYouTubePlaylists(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(playlists, func() {
				rows := make([][]string, 0, len(playlists))
				for _, p := range playlists {
					rows = append(rows, []string{p.ID, p.Title, output.Dash(deref(p.Privacy)), boolLabel(p.IsDefault, "yes", "no")})
				}
				printer.Table([]string{"id", "title", "privacy", "default"}, rows)
			})
		},
	}
}

func newYouTubeCreatePlaylistCmd(state *State) *cobra.Command {
	var description, privacy string
	cmd := &cobra.Command{
		Use:   "create-playlist <account-id> <title>",
		Short: "Create a playlist on the connected channel",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			playlist, err := client.Accounts.CreateYouTubePlaylist(cmd.Context(), args[0],
				&fopost.CreateYouTubePlaylistRequest{
					Title: args[1], Description: description, Privacy: privacy,
				})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(playlist, func() {
				printer.Table([]string{"id", "title"}, [][]string{{playlist.ID, playlist.Title}})
			})
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "Playlist description")
	cmd.Flags().StringVar(&privacy, "privacy", "", "public, unlisted or private")
	return cmd
}

func newYouTubeSetDefaultPlaylistCmd(state *State) *cobra.Command {
	var clear bool
	cmd := &cobra.Command{
		Use:   "set-default-playlist <account-id> [playlist-id]",
		Short: "Set the playlist a new video joins when a post does not pick one",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			playlistID := ""
			if !clear {
				if len(args) < 2 {
					return errMissingPlaylist
				}
				playlistID = args[1]
			}
			result, err := client.Accounts.SetDefaultYouTubePlaylist(cmd.Context(), args[0], playlistID)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(map[string]string{"playlist_id": result}, func() {
				printer.Table([]string{"playlist id"}, [][]string{{output.Dash(result)}})
			})
		},
	}
	cmd.Flags().BoolVar(&clear, "clear", false, "Clear the stored default instead of setting one")
	return cmd
}

func newYouTubeCaptionsCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "captions <account-id> <video-id>",
		Short: "List the caption tracks on one of the channel's videos",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			tracks, err := client.Accounts.ListYouTubeCaptions(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(tracks, func() {
				rows := make([][]string, 0, len(tracks))
				for _, t := range tracks {
					rows = append(rows, []string{t.ID, t.Language, t.Name, boolLabel(t.IsDraft, "yes", "no")})
				}
				printer.Table([]string{"id", "language", "name", "draft"}, rows)
			})
		},
	}
}

func newYouTubeTranscriptCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "transcript <account-id> <caption-id>",
		Short: "Print one caption track as text",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			transcript, err := client.Accounts.ReadYouTubeTranscript(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(transcript, func() {
				printer.Line("%s", transcript.Transcript)
			})
		},
	}
}

func newAccountsBlueskyCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bluesky",
		Short: "Read and set a Bluesky connection's default post languages",
	}
	cmd.AddCommand(newBlueskyLanguagesCmd(state), newBlueskySetLanguagesCmd(state))
	return cmd
}

func newBlueskyLanguagesCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "languages <account-id>",
		Short: "Show the default post languages",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Accounts.GetBlueskyLanguages(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printLanguages(state, result)
		},
	}
}

func newBlueskySetLanguagesCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "set-languages <account-id> [tag...]",
		Short: "Set the default post languages; no tags clears them",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			result, err := client.Accounts.SetBlueskyLanguages(cmd.Context(), args[0], args[1:])
			if err != nil {
				return err
			}
			return printLanguages(state, result)
		},
	}
}

func printLanguages(state *State, result *fopost.BlueskyLanguages) error {
	printer := state.Printer()
	return printer.Value(result, func() {
		rows := make([][]string, 0, len(result.Languages))
		for _, tag := range result.Languages {
			rows = append(rows, []string{tag})
		}
		printer.Table([]string{"language"}, rows)
	})
}

func newAccountsTikTokCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tiktok",
		Short: "Read TikTok creator limits and search its music, places and videos",
	}
	cmd.AddCommand(
		newTikTokCreatorInfoCmd(state),
		newTikTokMusicCmd(state),
		newTikTokLocationsCmd(state),
		newTikTokVideoCmd(state),
	)
	return cmd
}

func newTikTokCreatorInfoCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "creator-info <account-id>",
		Short: "Show the switches TikTok enforces at publish time",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			info, err := client.Accounts.GetTikTokCreatorInfo(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(info, func() {
				duration := "-"
				if info.MaxVideoPostDurationSec != nil {
					duration = strconv.Itoa(*info.MaxVideoPostDurationSec) + "s"
				}
				printer.Table(
					[]string{"comments", "duet", "stitch", "max video"},
					[][]string{{
						boolLabel(info.CommentDisabled, "off", "on"),
						boolLabel(info.DuetDisabled, "off", "on"),
						boolLabel(info.StitchDisabled, "off", "on"),
						duration,
					}},
				)
			})
		},
	}
}

func newTikTokMusicCmd(state *State) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "music <account-id> <query>",
		Short: "Search TikTok's Commercial Music Library",
		Long:  "Searches TikTok's Commercial Music Library. Needs the Marketing API product on\nthe TikTok app; without it the API answers 403 naming what to enable.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			tracks, err := client.Accounts.SearchTikTokMusic(cmd.Context(), args[0],
				fopost.TikTokSearchOptions{Query: args[1], Limit: limit})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(tracks, func() {
				rows := make([][]string, 0, len(tracks))
				for _, t := range tracks {
					rows = append(rows, []string{t.ID, t.Title, output.Dash(deref(t.Author))})
				}
				printer.Table([]string{"id", "title", "author"}, rows)
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "How many to return, 1 to 50")
	return cmd
}

func newTikTokLocationsCmd(state *State) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "locations <account-id> <query>",
		Short: "Search the places a TikTok post can be tagged with",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			places, err := client.Accounts.SearchTikTokLocations(cmd.Context(), args[0],
				fopost.TikTokSearchOptions{Query: args[1], Limit: limit})
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(places, func() {
				rows := make([][]string, 0, len(places))
				for _, p := range places {
					rows = append(rows, []string{p.ID, p.Name, output.Dash(deref(p.City))})
				}
				printer.Table([]string{"id", "name", "city"}, rows)
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "How many to return, 1 to 50")
	return cmd
}

func newTikTokVideoCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "video <account-id> <share-url>",
		Short: "Resolve a share link to one of this account's own videos",
		Long:  "Resolves a share link for repurposing. TikTok serves no raw media file, so the\ndownload url is the share address, which is what a repurpose run reads.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			video, err := client.Accounts.LookupTikTokVideo(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(video, func() {
				printer.Table([]string{"video id", "title", "download url"},
					[][]string{{video.VideoID, output.Dash(deref(video.Title)), output.Dash(deref(video.DownloadURL))}})
			})
		},
	}
}

func newAccountsInstagramCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instagram",
		Short: "Search Reel audio and read the publishing limit and live stories",
	}
	cmd.AddCommand(
		newInstagramAudioCmd(state),
		newInstagramPublishingLimitCmd(state),
		newInstagramStoriesCmd(state),
	)
	return cmd
}

func newInstagramAudioCmd(state *State) *cobra.Command {
	var audioType string
	cmd := &cobra.Command{
		Use:   "audio <account-id> [query]",
		Short: "Search the tracks a Reel can carry",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			opts := &fopost.InstagramAudioSearchOptions{AudioType: audioType}
			if len(args) > 1 {
				opts.Query = args[1]
			}
			tracks, err := client.Accounts.SearchInstagramAudio(cmd.Context(), args[0], opts)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(tracks, func() {
				rows := make([][]string, 0, len(tracks))
				for _, t := range tracks {
					rows = append(rows, []string{t.ID, output.Dash(deref(t.Title)), output.Dash(deref(t.Artist))})
				}
				printer.Table([]string{"id", "title", "artist"}, rows)
			})
		},
	}
	cmd.Flags().StringVar(&audioType, "audio-type", "", "music or original_sound")
	return cmd
}

func newInstagramPublishingLimitCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "publishing-limit <account-id>",
		Short: "Show how many posts are left before Instagram refuses the next one",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			limit, err := client.Accounts.GetInstagramPublishingLimit(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(limit, func() {
				printer.Table([]string{"used", "total", "remaining"}, [][]string{{
					strconv.Itoa(limit.QuotaUsage),
					intLabel(limit.QuotaTotal),
					intLabel(limit.Remaining),
				}})
			})
		},
	}
}

func newInstagramStoriesCmd(state *State) *cobra.Command {
	var insights bool
	cmd := &cobra.Command{
		Use:   "stories <account-id>",
		Short: "List the stories still inside their 24 hours",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			stories, err := client.Accounts.ListInstagramStories(cmd.Context(), args[0], insights)
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(stories, func() {
				rows := make([][]string, 0, len(stories))
				for _, s := range stories {
					rows = append(rows, []string{s.ID, output.Dash(deref(s.MediaType)), output.Dash(deref(s.Permalink))})
				}
				printer.Table([]string{"id", "type", "permalink"}, rows)
			})
		},
	}
	cmd.Flags().BoolVar(&insights, "insights", false, "Fetch each story's insights, at one extra call per story")
	return cmd
}

func newAccountsLinkedInCmd(state *State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "linkedin",
		Short: "Find the organizations a LinkedIn post can mention",
	}
	cmd.AddCommand(newLinkedInMentionsCmd(state))
	return cmd
}

func newLinkedInMentionsCmd(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "mentions <account-id> <query>",
		Short: "Search organizations and print the annotation to paste",
		Long:  "People are not searchable: LinkedIn has no public person search, so a member\nmention needs a URN you already hold.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := state.Client()
			if err != nil {
				return err
			}
			mentions, err := client.Accounts.SearchLinkedInMentions(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			printer := state.Printer()
			return printer.Value(mentions, func() {
				rows := make([][]string, 0, len(mentions))
				for _, m := range mentions {
					rows = append(rows, []string{m.Name, m.URN, m.Annotation})
				}
				printer.Table([]string{"name", "urn", "annotation"}, rows)
			})
		},
	}
}
