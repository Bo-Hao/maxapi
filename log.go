package maxapi

import (
	"bytes"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

func LogDebugToDailyLogFile(args ...interface{}) error {
	today := time.Now().Format("2006-01-02")
	var buffer bytes.Buffer
	buffer.WriteString("log/")
	buffer.WriteString(today)
	buffer.WriteString(".log")
	file, err := os.OpenFile(buffer.String(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	logrus.SetOutput(file)
	formatter := &logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.999",
	}
	logrus.SetFormatter(formatter)
	logrus.Debug(args)
	file.Close()
	return nil
}

func LogInfoToDailyLogFile(args ...interface{}) error {
	today := time.Now().Format("2006-01-02")
	var buffer bytes.Buffer
	buffer.WriteString("log/")
	buffer.WriteString(today)
	buffer.WriteString(".log")
	file, err := os.OpenFile(buffer.String(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	logrus.SetOutput(file)
	formatter := &logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.999",
	}
	logrus.SetFormatter(formatter)
	logrus.Info(args)
	file.Close()
	return nil
}

func LogWarningToDailyLogFile(args ...interface{}) error {
	today := time.Now().Format("2006-01-02")
	var buffer bytes.Buffer
	buffer.WriteString("log/")
	buffer.WriteString(today)
	buffer.WriteString(".log")
	file, err := os.OpenFile(buffer.String(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	logrus.SetOutput(file)
	formatter := &logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.999",
	}
	logrus.SetFormatter(formatter)
	logrus.Warning(args)
	file.Close()
	return nil
}

func LogErrorToDailyLogFile(args ...interface{}) error {
	today := time.Now().Format("2006-01-02")
	var buffer bytes.Buffer
	buffer.WriteString("log/")
	buffer.WriteString(today)
	buffer.WriteString(".log")
	file, err := os.OpenFile(buffer.String(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	logrus.SetOutput(file)
	formatter := &logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.999",
	}
	logrus.SetFormatter(formatter)
	logrus.Error(args)
	file.Close()
	return nil
}

func LogFatalToDailyLogFile(args ...interface{}) error {
	today := time.Now().Format("2006-01-02")
	var buffer bytes.Buffer
	buffer.WriteString("log/")
	buffer.WriteString(today)
	buffer.WriteString(".log")
	file, err := os.OpenFile(buffer.String(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	logrus.SetOutput(file)
	formatter := &logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.999",
	}
	logrus.SetFormatter(formatter)
	logrus.Fatal(args)
	file.Close()
	return nil
}

func LogPanicToDailyLogFile(args ...interface{}) error {
	today := time.Now().Format("2006-01-02")
	var buffer bytes.Buffer
	buffer.WriteString("log/")
	buffer.WriteString(today)
	buffer.WriteString(".log")
	file, err := os.OpenFile(buffer.String(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	logrus.SetOutput(file)
	formatter := &logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.999",
	}
	logrus.SetFormatter(formatter)
	logrus.Panic(args)
	file.Close()
	return nil
}
