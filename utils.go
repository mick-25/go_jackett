package main

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/daniwalter001/jackett_fiber/types"
	"github.com/gofiber/fiber/v2"
)

func getMeta(id string, type_ string) (string, string) {

	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)

	splitedId := strings.Split(id, ":")
	api := "https://v3-cinemeta.strem.io/meta/" + type_ + "/" + splitedId[0] + ".json"
	fmt.Println(api)
	request := fiber.Get(api)

	status, data, err := request.Bytes()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Status code: %d\n", status)

	if status >= 400 {
		return "", ""
	}

	var res types.IMDBMeta

	jsonErr := json.Unmarshal(data, &res)

	if jsonErr != nil {
		panic(jsonErr)
	}

	return *res.Meta.Name, *res.Meta.Year
}

func getImdbFromKitsu(id string) []string {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)

	splitedId := strings.Split(id, ":")
	api := "https://anime-kitsu.strem.fun/meta/anime/" + splitedId[0] + ":" + splitedId[1] + ".json"
	request := fiber.Get(api)
	status, data, err := request.Bytes()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Status code: %d\n", status)

	if status >= 400 {
		return make([]string, 0)
	}

	var res types.KitsuMeta

	jsonErr := json.Unmarshal(data, &res)

	if jsonErr != nil {
		panic(jsonErr)
	}

	imdb := res.Meta.ImdbID
	var meta types.Videos

	for i := 0; i < len(res.Meta.Videos); i++ {
		a := res.Meta.Videos[i]
		// fmt.Println("-------------------------")
		// fmt.Println(a.ID)
		// fmt.Println(a.Episode)
		// fmt.Println(a.Season)
		// fmt.Println(a.ImdbSeason)
		// fmt.Println(a.ImdbEpisode)
		// fmt.Println("-------------------------")

		if a.ID == id {
			meta = res.Meta.Videos[i]
		}
	}
	var resArray []string

	var e int
	var abs string

	if meta.Episode != meta.ImdbSeason || meta.ImdbSeason == 1 {
		abs = "true"
	} else {
		abs = "false"
	}

	if meta.ImdbSeason == 1 {
		e = meta.ImdbEpisode
	} else {
		e = meta.Episode
	}

	resArray = append(resArray, imdb, fmt.Sprint(meta.ImdbSeason), fmt.Sprint(meta.ImdbEpisode), fmt.Sprint(meta.Season), fmt.Sprint(e), abs)

	return resArray

}

func fetchTorrent(query string, type_ string) []types.ItemsParsed {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)

	host := "http://37.187.141.48:9117"
	apiKey := "10f9a9qd7wtq1lxutduyxz33zprl2gri"
	// host := "http://64.217.144.218:9117"
	// apiKey := "34m05y3vdqnbiauaenqlp7rqj2g8ovz3"
	category := ""
	if type_ == "movie" {
		category = ""
	}
	query = strings.ReplaceAll(query, " ", "+")

	api := fmt.Sprintf("%s/api/v2.0/indexers/yggtorrent/results/torznab/api?apikey=%s&cat=%s&q=%s&cache=false", host, apiKey, category, query)

	// api := fmt.Sprintf("%s/api/v2.0/indexers/thepiratebay/results/torznab/api?apikey=%s&t=search&cat=%s&q=%s&cache=false", host, apiKey, category, query)

	// http://64.217.144.218:9117/api/v2.0/indexers/thepiratebay/results/torznab/api?apikey=34m05y3vdqnbiauaenqlp7rqj2g8ovz3&t=search&cat=&q=
	// http://64.217.144.218:9117/api/v2.0/indexers/nyaasi/results/torznab/api?apikey=34m05y3vdqnbiauaenqlp7rqj2g8ovz3&t=search&cat=&q=

	request := fiber.Get(api)

	status, data, err := request.Bytes()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Status code: %d\n", status)
	if status >= 400 {
		return make([]types.ItemsParsed, 0)
	}

	var res types.JackettRssReponse

	xmlErr := xml.Unmarshal(data, &res)

	if xmlErr != nil {
		panic(xmlErr)
	}

	items := res.Channel.Item
	var parsedItems []types.ItemsParsed
	for i := 0; i < len(items); i++ {
		var a types.ItemsParsed
		a.Title = items[i].Title
		a.Link = items[i].Enclosure.URL
		a.Tracker = items[i].Jackettindexer.Text
		a.MagnetURI = items[i].Link
		attr := items[i].Attr
		for ii := 0; ii < len(attr); ii++ {
			if attr[ii].Name == "seeders" {
				a.Seeders = attr[ii].Value
			}
			if attr[ii].Name == "peers" {
				a.Peers = attr[ii].Value
			}
		}
		parsedItems = append(parsedItems, a)
	}

	// fmt.Println(PrettyPrint(parsedItems))

	return parsedItems
}

func readTorrent(item types.ItemsParsed) types.ItemsParsed {
	url := item.MagnetURI
	c := TorrentClient()
	defer c.Close()

	fileName := strings.ReplaceAll((strings.Split(url, "&file="))[len(strings.Split(url, "&file="))-1], "+", "_") + ".torrent"

	if len(fileName) > 100 {
		fileName = base64.StdEncoding.EncodeToString([]byte(fileName)) + ".torrent"

		fileName = fileName[len(fileName)-100:]
	}
	// fmt.Printf("Name: %s\n", fileName)
	file, errFile := os.Create(fmt.Sprintf("./temp/%s", fileName))

	if errFile != nil {
		fmt.Println(errFile)
		fmt.Printf("1Removing...%s\n", file.Name())
		os.Remove(file.Name())
		return item
	}

	request := fiber.Get(url).Timeout(5 * time.Second)

	status, data, err := request.Bytes()

	if status >= 400 {
		// fmt.Printf("1.5Removing...%d %s\n", status, file.Name())
		os.Remove(file.Name())
		return item
	}

	if err != nil {
		fmt.Printf("%s\n", err)
		// fmt.Printf("2Removing...%s\n", file.Name())
		os.Remove(file.Name())
		return item
	}

	if err != nil {
		fmt.Println(err)
		// fmt.Printf("3Removing...%s\n", file.Name())
		os.Remove(file.Name())
		return item
	}
	// _, fileError := io.Copy(file, response.Body)

	fileError := os.WriteFile(file.Name(), data, 0666)

	if fileError != nil {
		fmt.Println(fileError)
		fmt.Printf("4Removing...%s\n", file.Name())
		os.Remove(file.Name())
		return item
	}
	// fmt.Printf("Name2: %s\n", file.Name())

	t, addErr := c.AddTorrentFromFile(file.Name())
	// <-t.GotInfo()

	if addErr != nil {
		fmt.Printf("Err: %s\n", addErr)
		fmt.Printf("5Removing...%s\n", file.Name())
		os.Remove(file.Name())
		return item
	}
	// fmt.Printf("6Removing...%s\n", file.Name())
	os.Remove(file.Name())

	var files []torrent.File

	for i := 0; i < len(t.Files()); i++ {
		file := t.Files()[i]
		files = append(files, *file)
	}
	item.TorrentData = files

	return item

}

func readTorrentFromMagnet(item types.ItemsParsed) types.ItemsParsed {

	c := TorrentClient()

	t, addErr := c.AddMagnet(item.MagnetURI)

	if addErr != nil {
		fmt.Printf("ErrMagnet: %s\n", addErr)
		return item
	}

	// <-t.GotInfo()

	ed := make(chan string, 1)
	go func() {
		<-t.GotInfo()
		ed <- "okok"
	}()

	select {
	case <-time.After(15 * time.Second):
		// fmt.Printf("%s => %s\n", item.Title, "Timeout1")
		return item
	case res := <-ed:
		if res == "okok" {
			var files []torrent.File
			// fmt.Printf("%s =2> %d\n", item.Title, len(t.Files()))
			for i := 0; i < len(t.Files()); i++ {
				file := t.Files()[i]
				files = append(files, *file)
			}
			item.TorrentData = files
		}
		return item
	}

}
