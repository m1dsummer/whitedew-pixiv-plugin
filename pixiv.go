package pixiv_plugin

import (
	"encoding/base64"
	"encoding/json"
	"github.com/m1dsummer/whitedew"
	"github.com/parnurzeal/gorequest"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

type PluginPixiv struct {}

var images []string
var todoPull chan string
var done chan bool

func (p PluginPixiv)Init(w *whitedew.WhiteDew) {
	// 使用 5 个协程并发下载图片
	todoPull = make(chan string, 5)
	done = make(chan bool)
	w.SetRowMsgHandler(rowMegHandler)
	cachePictures(w)
}

func cachePictures(w *whitedew.WhiteDew) {
	cacheDir := path.Join(w.Config.CacheDir, "pixiv_images")
	dirInfo, err := os.Stat(cacheDir)
	if os.IsNotExist(err) {
		log.Println("cache directory not existed, creating it: "+cacheDir)
		err2 := os.MkdirAll(cacheDir, os.ModePerm)
		if err2 != nil {
			log.Println(err2)
			return
		}
	} else {
		if !dirInfo.IsDir() {
			log.Println(cacheDir+" should be a directory")
			return
		}
	}

	files,err := ioutil.ReadDir(cacheDir)
	if err != nil {
		log.Println(err)
		return
	}
	if len(files) != 0 {
		for _,fileInfo := range files {
			relativePath := path.Join(cacheDir,fileInfo.Name())
			//absPath,_ := filepath.Abs(relativePath)
			images = append(images, relativePath)
		}
		log.Println("using cache for pixiv plugin.")
		return
	}

	_,body,errs := gorequest.New().Get("https://pix.ipv4.host/ranks?page=1&date=2021-06-27&mode=month&pageSize=30").EndBytes()
	if errs != nil {
		log.Println(errs)
		return
	}

	var tmp map[string]interface{}
	err = json.Unmarshal(body, &tmp)
	if err != nil {
		log.Println(err)
		return
	}

	imageList := tmp["data"].([]interface{})
	go downloadAndSave(w)
	for _,image := range imageList {
		info := image.(map[string]interface{})
		for _,urls := range info["imageUrls"].([]interface{}) {
			imageUrl := urls.(map[string]interface{})["large"].(string)
			imageUrl = strings.ReplaceAll(imageUrl, "https://i.pximg.net/", "https://acgpic.net/")
			todoPull <- imageUrl
		}
	}
	close(todoPull)
	<-done
}

func downloadAndSave(w *whitedew.WhiteDew) {
	cacheDir := w.Config.CacheDir

	for  {
		time.Sleep(100*time.Millisecond)
		imageUrl,ok := <-todoPull
		if !ok {
			done <- true
			break
		}
		go func () {
			log.Println("downloading: "+imageUrl)
			request := gorequest.New().
				Get(imageUrl).
				Set("referer","https://pixivic.com/")
			resp, _, errs := request.EndBytes()
			if errs != nil {
				log.Println(errs)
				return
			}
			if resp.StatusCode != 200 {
				log.Println(imageUrl, resp.StatusCode)
				return
			}

			urlInfo,_ := url.Parse(imageUrl)
			index := strings.LastIndex(urlInfo.Path, "/")
			filename := urlInfo.Path[index+1:]
			filename = strings.ReplaceAll(filename, ".jpg", ".webp")
			fileLocation := path.Join(cacheDir,"pixiv_images",filename)
			fp,_ := os.Create(fileLocation)
			_,_ = io.Copy(fp, resp.Body)
			log.Println(fp.Name())
			_ = fp.Close()

			images = append(images, fileLocation)
		}()
	}
}

func rowMegHandler(s *whitedew.Session) {
	log.Println("row message handler")
	msg := s.Message.GetContent()
	cmd := "色图"
	if strings.Contains(msg, cmd) {
		sendImage(s)
	}
}

func sendImage(s *whitedew.Session) {
	msgInfo := whitedew.AnalyzeMsg(s.Message)
	image := images[rand.Intn(len(images))]
	data,_ := ioutil.ReadFile(image)
	imageBase64 := "base64://"+base64.StdEncoding.EncodeToString(data)
	if s.Message.GetMsgType() == "group" {
		chain := whitedew.MessageChain{}
		if msgInfo.IsAtMessage && msgInfo.At == s.Message.GetSelfId() {
			msgStr := chain.Prepare().At(s.Sender.GetId()).Image(imageBase64).String()
			s.PostGroupMessage(s.Message.(whitedew.GroupMessage).GroupId, msgStr)
		}
	} else {
		chain := whitedew.MessageChain{}
		msgStr := chain.Prepare().Image(imageBase64).String()
		s.PostPrivateMessage(s.Sender.GetId(), msgStr)
	}
}
