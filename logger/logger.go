package logger

import (
	"log"
	"os"
)

var infoLogger *log.Logger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
var debugLogger *log.Logger = log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime)
var errorLogger *log.Logger = log.New(os.Stdout, "ERROR: ", log.Ldate|log.Ltime)

func Infoln(v ...interface{}) {
	infoLogger.Println(v...)
}

func Infof(s string, v ...interface{}) {
	infoLogger.Printf(s, v...)
}

func Debugln(v ...interface{}) {
	debugLogger.Println(v...)
}

func Debugf(s string, v ...interface{}) {
	debugLogger.Printf(s, v...)
}

func Errorln(v ...interface{}) {
	errorLogger.Fatalln(v...)
}

func Errorf(s string, v ...interface{}) {
	errorLogger.Fatalf(s, v...)
}
