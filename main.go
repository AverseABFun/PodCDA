package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"

	gtts "github.com/Duckduckgot/gtts"
	graph "github.com/dominikbraun/graph"
)

const RedirectTypeTimestamp = "timestamp"
const RedirectTypeSkip = "skip"

const SorterTypeNone = "none"
const SorterTypeShortestSkip = "shortest_skip"

type overrides struct {
	Options_prefix                  string
	Options_item_separator          string
	Last_options_item_separator     string
	Speech_options_separator        string
	Options_seconds_prefix          string
	Options_seconds_forward_prefix  string
	Options_seconds_backward_prefix string
	Options_timestamp_go_to         string
	Options_timestamp_go_to_suffix  string
	Options_seconds_suffix_plural   string
	Options_seconds_suffix_singular string
	Request_to_pause                string
}

var currentOverrides = overrides{
	Options_prefix:                  "You can ",
	Options_item_separator:          ", ",
	Last_options_item_separator:     ", or ",
	Speech_options_separator:        " ... ... ... ",
	Options_seconds_prefix:          "To ",
	Options_seconds_forward_prefix:  ", skip forward ",
	Options_seconds_backward_prefix: ", skip backward ",
	Options_timestamp_go_to:         ", go to timestamp ",
	Options_timestamp_go_to_suffix:  ". ",
	Options_seconds_suffix_plural:   " seconds. ",
	Options_seconds_suffix_singular: " seconds. ",
	Request_to_pause:                "Please pause and make your decision now. ",
}

type ConversionManifest struct {
	Version      int       // Version of the manifest, currently only version 1 is supported
	Path         string    // Path to the manifest generated by the CDAdventure compiler
	OutputPath   string    // Path to the output audio file
	Preamble     string    // Path to the preamble manifest
	Overrides    overrides // Overrides for the default text
	RedirectType string    // Redirect type(i.e. timestamp or skip)
	Sorter       string    // Sorter type(i.e. none or shortest_skip)
	LastEnd      string    // ID of the end track that should be sorted to the end(not used with sorter none)
}

type Preamble struct {
	Version               int     // Version of the preamble, currently only version 1 is supported
	Uses_file             bool    // Whether the preamble uses a separate audio file
	Audio_file            string  // Path to the audio file if the preamble uses one
	Merge                 bool    // Whether the speech and audio file should be merged
	Audio_file_volume     float64 // Volume of the audio file, if the preamble uses a separate audio file and Merge is set. 0.0 is silent, 1.0 is the original volume
	Speech                string  // Speech for the preamble, if the preamble does not use a separate audio file and Merge is not set
	Post_speech           string  // Speech for after the entire game
	Starting_speech_delay float64 // Delay after the preamble before the game starts
}

type Track struct {
	ID             string         // ID of the track
	OriginalSpeech string         // Speech of the track
	Title          string         // Title of the track
	Options        map[string]int // Options of the track
	End            bool           // Whether the track is the end of the game(there can be more then one end)
	NoAppend       bool           // Identical to End
}

type IDTrack struct {
	ID             string            // ID of the track
	OriginalSpeech string            // Speech of the track
	Title          string            // Title of the track
	Options        map[string]string // Options of the track
	End            bool              // Whether the track is the end of the game(there can be more then one end)
}

func trackHash(t Track) string {
	return t.ID
}

type Meta struct {
	Name      string  // Name of the game
	Author    string  // Author of the game
	Beginning string  // ID of the starting track of the game
	Version   float64 // Version of the game, should only be 1.3+
}

type CDAdventureManifest struct {
	Version int     // Version of the manifest, currently only version 1 is supported
	Meta    Meta    // Meta information of the game(author, name, version, etc.)
	Tracks  []Track // Tracks of the game
}

type CDAdventure struct {
	ConversionManifest ConversionManifest
	Preamble           Preamble
	Manifest           CDAdventureManifest
}

func CheckConversionManifest(manifest ConversionManifest) (bool, string) {
	var conversionPath = filepath.Dir(flag.Arg(0))
	if manifest.Version != 1 {
		return false, "Invalid manifest version"
	}
	if manifest.Path == "" {
		return false, "Invalid manifest path"
	}
	var _, openErr1 = os.Open(filepath.Join(conversionPath, manifest.Path))
	if openErr1 != nil {
		if os.IsNotExist(openErr1) {
			return false, "Manifest file does not exist"
		}
		return false, "Error opening manifest file"
	}
	if manifest.OutputPath == "" {
		return false, "Invalid output path"
	}
	if manifest.Preamble == "" {
		return false, "Invalid preamble path"
	}
	var _, openErr2 = os.Open(filepath.Join(conversionPath, manifest.Preamble))
	if openErr2 != nil {
		if os.IsNotExist(openErr2) {
			return false, "Preamble file does not exist"
		}
		return false, "Error opening preamble file"
	}
	if manifest.RedirectType != RedirectTypeTimestamp && manifest.RedirectType != RedirectTypeSkip {
		return false, "Invalid redirect type"
	}
	if manifest.Sorter != SorterTypeNone && manifest.Sorter != SorterTypeShortestSkip {
		return false, "Invalid sorter type"
	}
	return true, ""
}

func CheckPreamble(preamble Preamble) (bool, string) {
	if preamble.Version != 1 {
		return false, "Invalid preamble version"
	}
	if preamble.Uses_file {
		var _, openErr = os.Open(preamble.Audio_file)
		if openErr != nil {
			if os.IsNotExist(openErr) {
				return false, "Audio file does not exist"
			}
			return false, "Error opening audio file"
		}
	}
	if preamble.Merge {
		if !preamble.Uses_file {
			return false, "Preamble wants to merge but does not use a separate audio file"
		}
	}
	if preamble.Speech == "" {
		return false, "Invalid speech"
	}
	if preamble.Post_speech == "" {
		return false, "Invalid post speech"
	}
	if preamble.Starting_speech_delay == 0 {
		preamble.Starting_speech_delay = 5.0
	}
	if preamble.Starting_speech_delay < 0 {
		return false, "Invalid starting speech delay"
	}
	return true, ""
}

func CheckCDAdventureManifest(manifest CDAdventureManifest) (bool, string) {
	if manifest.Version != 1 {
		return false, "Invalid manifest version"
	}
	if manifest.Meta.Name == "" {
		return false, "Invalid game name"
	}
	if manifest.Meta.Author == "" {
		return false, "Invalid game author"
	}
	if manifest.Meta.Beginning == "" {
		return false, "Invalid beginning track"
	}
	var beginningTrackFound = false
	for _, track := range manifest.Tracks {
		if track.ID == manifest.Meta.Beginning {
			beginningTrackFound = true
			break
		}
	}
	if !beginningTrackFound {
		return false, "Beginning track not found"
	}
	for i, track := range manifest.Tracks {
		if track.ID == "" {
			return false, "Invalid track ID at index " + string(i)
		}
		if track.OriginalSpeech == "" {
			return false, "Invalid track speech at track " + track.ID
		}
		if track.Title == "" {
			return false, "Invalid track title at track " + track.ID
		}
		if len(track.Options) == 0 && !track.End {
			return false, "Invalid track options at track " + track.ID
		}
		for option := range track.Options {
			if option == "" {
				return false, "Invalid option ID at track " + track.ID
			}
		}
	}
	return true, ""
}

func GetTrackByID(tracks []Track, id string) (bool, Track) {
	for _, track := range tracks {
		if track.ID == id {
			return true, track
		}
	}
	return false, Track{}
}

func GetIndexOfTrack(tracks []Track, id string) int {
	for i, track := range tracks {
		if track.ID == id {
			return i
		}
	}
	return -1
}

func AddTrackToTracksNoOverwrite(tracks []Track, track Track, index int) ([]Track, error) {
	var found, _ = GetTrackByID(tracks, track.ID)
	if !found {
		fmt.Println(track.ID)
		return tracks, fmt.Errorf("Track not found in tracks")
	}
	var indexOfTrack = GetIndexOfTrack(tracks, track.ID)
	if indexOfTrack == index {
		return tracks, nil
	}
	var newTracks []Track = tracks
	newTracks = append(newTracks[:indexOfTrack], track)
	newTracks = append(newTracks, tracks[indexOfTrack:]...)
	return newTracks, nil
}

func CreateIDTrack(track Track, cdAdventure CDAdventure) IDTrack {
	var idTrack = IDTrack{
		ID:             track.ID,
		OriginalSpeech: track.OriginalSpeech,
		Title:          track.Title,
		Options:        make(map[string]string),
		End:            track.End,
	}
	for option, optionID := range track.Options {
		var track = cdAdventure.Manifest.Tracks[optionID]
		idTrack.Options[option] = track.ID
	}
	return idTrack
}

func SortTracks(tracks []Track, data CDAdventure) []Track {
	if data.ConversionManifest.Sorter == SorterTypeNone {
		return tracks
	}
	var _, beginning = GetTrackByID(tracks, data.Manifest.Meta.Beginning)
	tracks, err := AddTrackToTracksNoOverwrite(tracks, beginning, 0)
	var IDTracks []IDTrack
	for _, track := range tracks {
		IDTracks = append(IDTracks, CreateIDTrack(track, data))
	}
	if err != nil {
		fmt.Println("Error sorting tracks:", err)
		return tracks
	}
	var trackGraph = graph.New(trackHash, graph.Directed(), graph.Weighted(), graph.Rooted())
	for _, track := range tracks {
		trackGraph.AddVertex(track)
	}
	for i, track := range tracks {
		for _, option := range track.Options {
			trackGraph.AddEdge(track.ID, tracks[option].ID, graph.EdgeWeight(int(math.Abs(float64(option-i)))))
		}
	}
	var tempOutput []Track
	graph.BFS(trackGraph, beginning.ID, func(trackID string) bool {
		var _, track = GetTrackByID(tracks, trackID)
		tempOutput = append(tempOutput, track)
		return false
	})
	var output []Track = make([]Track, len(tempOutput))
	for _, track := range IDTracks {
		for _, option := range track.Options {
			var i = GetIndexOfTrack(tempOutput, option)
			if i == -1 {
				fmt.Println("Error sorting tracks: track not found")
				return tracks
			}
			_, output[i] = GetTrackByID(tracks, option)
		}
	}
	return output
}

func CreateSpeech(trackID string, speech string) {
	var speechV = gtts.Speech{Folder: "out/temp", Language: "en", Handler: nil}
	speechV.CreateSpeechFile(speech, trackID+".mp3")
}

func GenerateSpeechFromTrack(track Track, conversionManifest ConversionManifest) string {
	var text = track.OriginalSpeech
	text += currentOverrides.Speech_options_separator
	text += currentOverrides.Options_prefix
	var i = 0
	for option, _ := range track.Options {
		if i != 0 && i != len(track.Options)-1 {
			text += currentOverrides.Options_item_separator
		} else if i == len(track.Options)-1 {
			text += currentOverrides.Last_options_item_separator
		}
		text += option
		i += 1
	}
	text += ". "
	for option, _ := range track.Options {
		text += currentOverrides.Options_seconds_prefix
		text += option
		if conversionManifest.RedirectType == RedirectTypeTimestamp {
			text += currentOverrides.Options_timestamp_go_to
		} else if conversionManifest.RedirectType == RedirectTypeSkip {
		}
	}
}

func main() {
	fmt.Println("PodCDA: Create a single audio file from a CDAdventure manifest")
	fmt.Println("Written by Arthur Beck (c) 2024")
	fmt.Println("Licensed under the GNU Affero General Public License v3.0")
	flag.Parse()
	var conversionPath string = flag.Arg(0)
	var _, openErr = os.Open(conversionPath)
	if openErr != nil {
		if os.IsNotExist(openErr) {
			fmt.Println("Err: conversion manifest does not exist")
			return
		}
		fmt.Println("Error opening conversion manifest:", openErr)
		return
	}
	var convData, readErr = os.ReadFile(conversionPath)
	if readErr != nil {
		fmt.Println("Error reading conversion manifest:", readErr)
		return
	}
	var conversionManifest ConversionManifest
	var jsonErr = json.Unmarshal(convData, &conversionManifest)
	if jsonErr != nil {
		fmt.Println("Error parsing conversion manifest:", jsonErr)
		return
	}
	var valid, errMsg = CheckConversionManifest(conversionManifest)
	if !valid {
		fmt.Println("Invalid conversion manifest:", errMsg)
		return
	}
	currentOverrides = conversionManifest.Overrides

	var preamblePath = filepath.Join(filepath.Dir(conversionPath), conversionManifest.Preamble)
	var preambleData, readErr2 = os.ReadFile(preamblePath)
	if readErr2 != nil {
		fmt.Println("Error reading preamble manifest:", readErr2)
		return
	}
	var preamble Preamble
	var jsonErr2 = json.Unmarshal(preambleData, &preamble)
	if jsonErr2 != nil {
		fmt.Println("Error parsing preamble manifest:", jsonErr2)
		return
	}
	var valid2, errMsg2 = CheckPreamble(preamble)
	if !valid2 {
		fmt.Println("Invalid preamble manifest:", errMsg2)
		return
	}
	var cdAdventurePath = filepath.Join(filepath.Dir(conversionPath), conversionManifest.Path)
	var cdAdventureData, readErr3 = os.ReadFile(cdAdventurePath)
	if readErr3 != nil {
		fmt.Println("Error reading CDAdventure manifest:", readErr3)
		return
	}
	var cdAdventureManifest CDAdventureManifest
	var jsonErr3 = json.Unmarshal(cdAdventureData, &cdAdventureManifest)
	if jsonErr3 != nil {
		fmt.Println("Error parsing CDAdventure manifest:", jsonErr3)
		return
	}
	var valid3, errMsg3 = CheckCDAdventureManifest(cdAdventureManifest)
	if !valid3 {
		fmt.Println("Invalid CDAdventure manifest:", errMsg3)
		return
	}
	var cdAdventure = CDAdventure{
		ConversionManifest: conversionManifest,
		Preamble:           preamble,
		Manifest:           cdAdventureManifest,
	}
	for _, track := range cdAdventure.Manifest.Tracks {
		if track.NoAppend {
			track.End = true
		}
	}
	cdAdventure.Manifest.Tracks = SortTracks(cdAdventure.Manifest.Tracks, cdAdventure)
	fmt.Println(cdAdventure.Manifest.Tracks)
}
