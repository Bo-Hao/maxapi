package maxapi

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

func LogWarningToDailyLogFile(message string) error {
	today := time.Now().Format("2006-01-02")
	var buffer bytes.Buffer
	buffer.WriteString("log/")
	buffer.WriteString(today)
	buffer.WriteString(".log")
	file, err := os.OpenFile(buffer.String(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	fmt.Println(buffer.String())

	logrus.SetOutput(file)
	formatter := &logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.999",
	}
	logrus.SetFormatter(formatter)
	logrus.Warning(message)
	file.Close()
	return nil
}
