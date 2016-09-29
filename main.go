package main

import (
	"log"
	"os"
	"path/filepath"
	"video_deploy/config"
)

var pathSeparator = string(filepath.Separator)

var shareDir string
var liveRootDir string

func init() {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatalln("Err Occure: Can not read Dir - ", os.Args[0])
	}

	encodingDir := filepath.Join(dir, "_log")
	if _, err := os.Stat(encodingDir); os.IsNotExist(err) {
		if err := os.Mkdir(encodingDir, 0754); err != nil {
			log.Println("fail to make directory:", encodingDir)
			log.Panicln(err.Error())
		} else {
			log.Println("make directory:", encodingDir)
		}
	}

	LOG_PATH := filepath.Join(encodingDir, "log")
	f, err := os.OpenFile(LOG_PATH, os.O_RDWR|os.O_CREATE|os.O_APPEND, os.ModeAppend)
	if err == nil {
		log.SetOutput(f)
	}

	config.Load()
	shareDir = config.Get("shareDir")
	liveRootDir = config.Get("liveRootDir")
}

func main() {
	log.Println("==================== Processing... ====================")
	deploy := setDeployInfo(os.Args[2])

	if _, err := os.Stat(deploy.getShareFilePath()); os.IsNotExist(err) {
		log.Fatalln("no such source file. program exit... :", err.Error())
	}

	isLinked := deploy.makeLink()
	if isLinked {
		encoding(deploy)
	}
	callComplate(os.Args[1], deploy.videoNo)
}
