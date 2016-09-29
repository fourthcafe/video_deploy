package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"video_deploy/config"
)

var encodingInfoMap = map[string]EncodingInfo{
	"720p": {size: "1280:-2", bitrate: "2500k"},
	"360p": {size: "640:-2", bitrate: "700k"},
	"180p": {size: "320:-2", bitrate: "400k"},
}

type EncodingInfo struct {
	size    string
	bitrate string
}

type Deploy struct {
	originalFile string
	categoryNo   string
	videoNo      string
	year         string
	month        string
	resolution   string
	fileName     string
	fileExt      string
}

func setDeployInfo(filename string) Deploy {
	tmp := strings.Split(filename, "-")
	fname := strings.Join(tmp[4:], "-")

	sizeIdx := strings.LastIndex(fname, "_")

	ret := Deploy{
		originalFile: filename,
		categoryNo:   tmp[0],
		videoNo:      tmp[1],
		year:         tmp[2],
		month:        tmp[3],
		resolution:   fname[sizeIdx+1 : strings.LastIndex(fname, ".")],
		fileName:     fname[:sizeIdx],
		fileExt:      filepath.Ext(fname),
	}

	return ret
}

func (this *Deploy) getShareFilePath() string {
	return filepath.Join(shareDir, this.originalFile)
}

func (this *Deploy) getLiveFileDir() string {
	return filepath.Join(liveRootDir, this.categoryNo, this.year, this.month)
}

func (this *Deploy) getLiveFilePath() string {
	return this.getLiveFilePathWithResolution(this.resolution)
}

func (this *Deploy) getLiveFilePathWithResolution(resolution string) string {
	return filepath.Join(
		this.getLiveFileDir(),
		fmt.Sprintf("%s_%s%s", this.fileName, resolution, this.fileExt))
}

/*
설정된 폴더로 파일을 Hard link 한다.
설정된 폴더에 동일한 파일이 있을 경우 md5 해쉬를 통해 값을 체크하여 동일하면 false,
다르면 파일을 Hard link 하며 true 를 리턴한다.
*/
func (this *Deploy) makeLink() bool {
	log.Printf("try file link: %s -> %s\n", this.getShareFilePath(), this.getLiveFilePath())

	if _, err := os.Stat(this.getLiveFileDir()); os.IsNotExist(err) {
		err := os.MkdirAll(this.getLiveFileDir(), 0754)
		if err != nil {
			log.Println("fail to make directory:", this.getLiveFileDir())
			log.Panicln(err.Error())
		}

		log.Println("make directory:", this.getLiveFileDir())
	}

	if _, err := os.Stat(this.getLiveFilePath()); err == nil {
		log.Println("same file exist in liveDir:", this.getLiveFilePath())

		isSame, err := isSameFile(this.getShareFilePath(), this.getLiveFilePath())
		if err != nil {
			log.Println("err in compare md5 hash:", err.Error())
		}

		if isSame {
			log.Println("hash is same. skip link & encoding:", this.getShareFilePath())
			return false
		} else {
			log.Println("hash is different. file link continue...")
			if err := os.Remove(this.getLiveFilePath()); err != nil {
				log.Println("remove err:", err.Error())
			}
		}
	}

	if err := os.Link(this.getShareFilePath(), this.getLiveFilePath()); err == nil {
		log.Printf("file link success")
		setPermission(this.getLiveFilePath())
	} else {
		log.Fatalln("err in Link:", err.Error())
	}

	return true
}

/*
	두 파일의 md5 hash 값을 비교하여 같으면 true, 다르면 false 를 돌려준다.
*/
func isSameFile(filePath, anoFilePath string) (bool, error) {
	d0, err := computeMD5Hash(filePath)
	if err != nil {
		return false, err
	}

	d1, err := computeMD5Hash(anoFilePath)
	if err != nil {
		return false, err
	}

	if hex.EncodeToString(d0) == hex.EncodeToString(d1) {
		return true, nil
	}

	return false, err
}

/*
	md5 hash 를 사용하여 파일의 hash 값을 얻는다.
*/
func computeMD5Hash(filePath string) ([]byte, error) {
	var result []byte
	file, err := os.Open(filePath)
	if err != nil {
		return result, err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return result, err
	}

	return hash.Sum(result), nil
}

func encoding(deploy Deploy) {
	switch deploy.resolution {
	case "1080p":
		output := ffmpegEncoding(deploy, deploy.resolution, "720p")
		setPermission(output)
		fallthrough
	case "720p":
		output := ffmpegEncoding(deploy, deploy.resolution, "360p")
		setPermission(output)
		fallthrough
	case "360p":
		output := ffmpegEncoding(deploy, deploy.resolution, "180p")
		setPermission(output)
	default:
		log.Fatalln("can not encoding file:", deploy.getLiveFilePath())
	}
}

/*
	인코딩 후 경로를 포함한 파일 이름을 반환한다.
*/
func ffmpegEncoding(deploy Deploy, sourceResolution, outputResolution string) string {
	start := time.Now()

	source := deploy.getLiveFilePathWithResolution(sourceResolution)
	output := deploy.getLiveFilePathWithResolution(outputResolution)
	encodingInfo := encodingInfoMap[outputResolution]

	cmd := exec.Command(config.Get("ffmpegExec"),
		"-i", source,
		"-vf",
		"scale="+encodingInfo.size,
		"-y",
		"-maxrate", encodingInfo.bitrate,
		"-b:a", "128k",
		"-ac", "2",
		output,
	)

	if err := cmd.Start(); err != nil {
		log.Printf("err in encoding: %s -> %s\n", source, output)
		log.Fatalln(err.Error())
	}

	log.Printf("encoding: %s -> %s\n", source, output)

	if err := cmd.Wait(); err != nil {
		log.Fatalln("err in exec wait:", err.Error())
	}

	log.Printf("complate: %s %s\n", output, time.Since(start).String())

	return output
}

/*
	OS가 리눅스인 경우 파일 권한을 755 로 설정한다.
*/
func setPermission(fileName string) {
	if runtime.GOOS == "linux" {
		log.Println("this OS is linux. set permission... ")
		permission := "755"

		err := exec.Command("chmod", permission, fileName).Run()
		if err == nil {
			log.Printf("complate set permission: %s %s\n", fileName, permission)

		} else {
			log.Printf("err in set permission: %s %s\n", fileName, permission)
		}
	}
}

/*
	지정된 작업이 완료되었음을 API 서버에 알린다.
*/
func callComplate(deviceId, videoNo string) {
	params :=
		url.Values{
			"device_id": {deviceId},
			"no":        {videoNo},
		}

	if res, err := http.PostForm(config.Get("apiURL"), params); err != nil {
		log.Println("err in callComplate:", err.Error())
		log.Println("URL:", config.Get("apiURL"))
		log.Println("params:", params)

	} else {
		body, _ := ioutil.ReadAll(res.Body)
		log.Println("Success callComplate")
		log.Println("URL:", config.Get("apiURL"))
		log.Println("params:", params)
		log.Println("result:", string(body))
	}

}
